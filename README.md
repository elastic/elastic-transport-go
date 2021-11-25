# elastic-transport-go

This library was lifted from elasticsearch-net and then transformed to be used across all Elastic services rather than
only Elasticsearch.

It provides the Transport interface used by `go-elasticsearch`, connection pool, cluster discovery, and multiple loggers.

## Installation

Add the package to your go.mod file:

`require github.com/elastic/elastic/transport-go/v8 main`

## Usage

### Transport
The transport provides the basic layer to access Elasticsearch APIs.

```go
package main

import (
	"log"
	"net/http"
	"net/url"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
)

func main() {
	u, _ := url.Parse("http://127.0.0.1:9200")

	cfg := elastictransport.Config{
		URLs: []*url.URL{u},
	}
	transport, err := elastictransport.New(cfg)
	if err != nil {
		log.Fatalln(err)
	}

	req, _ := http.NewRequest("GET", "/", nil)

	res, err := transport.Perform(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer res.Body.Close()

	log.Println(res)
}
```

> NOTE: It is _critical_ to both close the response body _and_ to consume it, in order to re-use persistent TCP connections in the default HTTP transport. If you're not interested in the response body, call `io.Copy(ioutil.Discard, res.Body)`.

### Discovery

Discovery module calls the cluster to retrieve its complete list of nodes.

Once your transport has been setup, you can easily trigger this behavior like so :

```go
err := transport.DiscoverNodes()
```

### Metrics

Allows you to retrieve metrics directly from the transport.

### Loggers

One of multiple loggers can be injected directly into the `Logger` configuration, these are as follow:

#### TextLogger
config:
```go
cfg := elastictransport.Config{
    elastictransport.TextLogger{os.Stdout, true, true},
}
```
output:
```
< {
<   "name" : "es",
<   "cluster_name" : "elasticsearch",
<   "cluster_uuid" : "RxB1iqTNT9q3LlIkTsmWRA",
<   "version" : {
<     "number" : "8.0.0-SNAPSHOT",
<     "build_flavor" : "default",
<     "build_type" : "docker",
<     "build_hash" : "0564e027dc6c69236937b1edcc04c207b4cd8128",
<     "build_date" : "2021-11-25T00:23:33.139514432Z",
<     "build_snapshot" : true,
<     "lucene_version" : "9.0.0",
<     "minimum_wire_compatibility_version" : "7.16.0",
<     "minimum_index_compatibility_version" : "7.0.0"
<   },
<   "tagline" : "You Know, for Search"
< }
```

#### JSONLogger
config:
```go
cfg := elastictransport.Config{
    Logger: &elastictransport.JSONLogger{os.Stdout, true, true},
}
```
output:
```json
{
  "@timestamp": "2021-11-25T16:33:51Z",
  "event": {
    "duration": 2892269
  },
  "url": {
    "scheme": "http",
    "domain": "127.0.0.1",
    "port": 9200,
    "path": "/",
    "query": ""
  },
  "http": {
    "request": {
      "method": "GET"
    },
    "response": {
      "status_code": 200,
      "body": "{\n  \"name\" : \"es1\",\n  \"cluster_name\" : \"go-elasticsearch\",\n  \"cluster_uuid\" : \"RxB1iqTNT9q3LlIkTsmWRA\",\n  \"version\" : {\n    \"number\" : \"8.0.0-SNAPSHOT\",\n    \"build_flavor\" : \"default\",\n    \"build_type\" : \"docker\",\n    \"build_hash\" : \"0564e027dc6c69236937b1edcc04c207b4cd8128\",\n    \"build_date\" : \"2021-11-25T00:23:33.139514432Z\",\n    \"build_snapshot\" : true,\n    \"lucene_version\" : \"9.0.0\",\n    \"minimum_wire_compatibility_version\" : \"8.0.0\",\n    \"minimum_index_compatibility_version\" : \"7.0.0\"\n  },\n  \"tagline\" : \"You Know, for Search\"\n}\n"
    }
  }
}
```

#### ColorLogger
config:
```go
cfg := elastictransport.Config{
    Logger: &elastictransport.ColorLogger{os.Stdout, true, true},
}
```
output:
```
GET http://127.0.0.1:9200/ 200 OK 2ms
« {
«   "name" : "es1",
«   "cluster_name" : "go-elasticsearch",
«   "cluster_uuid" : "RxB1iqTNT9q3LlIkTsmWRA",
«   "version" : {
«     "number" : "8.0.0-SNAPSHOT",
«     "build_flavor" : "default",
«     "build_type" : "docker",
«     "build_hash" : "0564e027dc6c69236937b1edcc04c207b4cd8128",
«     "build_date" : "2021-11-25T00:23:33.139514432Z",
«     "build_snapshot" : true,
«     "lucene_version" : "9.0.0",
«     "minimum_wire_compatibility_version" : "7.16.0",
«     "minimum_index_compatibility_version" : "7.0.0"
«   },
«   "tagline" : "You Know, for Search"
« }
────────────────────────────────────────────────────────────────────────────────
```

### CurlLogger
config:
```go
cfg := elastictransport.Config{
    Logger: &elastictransport.CurlLogger{os.Stdout, true, true},
}
```
output:
```shell
curl -X GET 'http://localhost:9200/?pretty'
# => 2021-11-25T16:40:11Z [200 OK] 3ms
# {
#  "name": "es1",
#  "cluster_name": "go-elasticsearch",
#  "cluster_uuid": "RxB1iqTNT9q3LlIkTsmWRA",
#  "version": {
#   "number": "8.0.0-SNAPSHOT",
#   "build_flavor": "default",
#   "build_type": "docker",
#   "build_hash": "0564e027dc6c69236937b1edcc04c207b4cd8128",
#   "build_date": "2021-11-25T00:23:33.139514432Z",
#   "build_snapshot": true,
#   "lucene_version": "9.0.0",
#   "minimum_wire_compatibility_version": "7.16.0",
#   "minimum_index_compatibility_version": "7.0.0"
#  },
#  "tagline": "You Know, for Search"
# }
```

# License

Licensed under the Apache License, Version 2.0.