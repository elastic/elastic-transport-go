package elastictransport

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"sync"
)

type gzipCompressor interface {
	// compress compresses the given io.ReadCloser and returns the gzip compressed data as a bytes.Buffer.
	compress(io.ReadCloser) (*bytes.Buffer, error)
	// collectBuffer collects the given bytes.Buffer for reuse.
	collectBuffer(*bytes.Buffer)
}

// simpleGzipCompressor is a simple implementation of gzipCompressor that creates a new gzip.Writer for each call.
type simpleGzipCompressor struct {
	compressionLevel int
}

func newSimpleGzipCompressor(compressionLevel int) gzipCompressor {
	return &simpleGzipCompressor{
		compressionLevel: compressionLevel,
	}
}

func (sg *simpleGzipCompressor) compress(rc io.ReadCloser) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, sg.compressionLevel)
	if err != nil {
		return nil, fmt.Errorf("failed setting up up compress request body (level %d): %s",
			sg.compressionLevel, err)
	}

	if _, err = io.Copy(zw, rc); err != nil {
		return nil, fmt.Errorf("failed to compress request body: %s", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to compress request body (during close): %s", err)
	}
	return &buf, nil
}

func (sg *simpleGzipCompressor) collectBuffer(buf *bytes.Buffer) {
	// no-op
}

type pooledGzipCompressor struct {
	gzipWriterPool   *sync.Pool
	bufferPool       *sync.Pool
	compressionLevel int
}

type gzipWriter struct {
	writer *gzip.Writer
	err    error
}

// newPooledGzipCompressor returns a new pooledGzipCompressor that uses a sync.Pool to reuse gzip.Writers.
func newPooledGzipCompressor(compressionLevel int) gzipCompressor {
	gzipWriterPool := sync.Pool{
		New: func() any {
			writer, err := gzip.NewWriterLevel(io.Discard, compressionLevel)
			return &gzipWriter{
				writer: writer,
				err:    err,
			}
		},
	}

	bufferPool := sync.Pool{
		New: func() any {
			return new(bytes.Buffer)
		},
	}

	return &pooledGzipCompressor{
		gzipWriterPool:   &gzipWriterPool,
		bufferPool:       &bufferPool,
		compressionLevel: compressionLevel,
	}
}

func (pg *pooledGzipCompressor) compress(rc io.ReadCloser) (*bytes.Buffer, error) {
	writer := pg.gzipWriterPool.Get().(*gzipWriter)
	defer pg.gzipWriterPool.Put(writer)

	if writer.err != nil {
		return nil, fmt.Errorf("failed setting up up compress request body (level %d): %s",
			pg.compressionLevel, writer.err)
	}

	buf := pg.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	writer.writer.Reset(buf)

	if _, err := io.Copy(writer.writer, rc); err != nil {
		return nil, fmt.Errorf("failed to compress request body: %s", err)
	}
	if err := writer.writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to compress request body (during close): %s", err)
	}
	return buf, nil
}

func (pg *pooledGzipCompressor) collectBuffer(buf *bytes.Buffer) {
	pg.bufferPool.Put(buf)
}
