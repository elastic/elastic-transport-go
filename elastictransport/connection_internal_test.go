// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

//go:build !integration
// +build !integration

package elastictransport

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestSingleConnectionPoolNext(t *testing.T) {
	t.Run("Single URL", func(t *testing.T) {
		pool := &singleConnectionPool{
			connection: &Connection{URL: &url.URL{Scheme: "http", Host: "foo1"}},
		}

		for i := 0; i < 7; i++ {
			c, err := pool.Next()
			if err != nil {
				t.Errorf("Unexpected error: %s", err)
			}

			if c.URL.String() != "http://foo1" {
				t.Errorf("Unexpected URL, want=http://foo1, got=%s", c.URL)
			}
		}
	})
}

func TestSingleConnectionPoolOnFailure(t *testing.T) {
	t.Run("Noop", func(t *testing.T) {
		pool := &singleConnectionPool{
			connection: &Connection{URL: &url.URL{Scheme: "http", Host: "foo1"}},
		}

		if err := pool.OnFailure(&Connection{URL: &url.URL{Scheme: "http", Host: "foo1"}}); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	})
}

func TestStatusConnectionPoolNext(t *testing.T) {
	t.Run("No URL", func(t *testing.T) {
		pool := &statusConnectionPool{}

		c, err := pool.Next()
		if err == nil {
			t.Errorf("Expected error, but got: %s", c.URL)
		}
	})

	t.Run("Two URLs", func(t *testing.T) {
		var c *Connection

		pool := &statusConnectionPool{
			live: []*Connection{
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo1"}},
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo2"}},
			},
			selector: &roundRobinSelector{curr: -1},
		}

		c, _ = pool.Next()

		if c.URL.String() != "http://foo1" {
			t.Errorf("Unexpected URL, want=foo1, got=%s", c.URL)
		}

		c, _ = pool.Next()
		if c.URL.String() != "http://foo2" {
			t.Errorf("Unexpected URL, want=http://foo2, got=%s", c.URL)
		}

		c, _ = pool.Next()
		if c.URL.String() != "http://foo1" {
			t.Errorf("Unexpected URL, want=http://foo1, got=%s", c.URL)
		}
	})

	t.Run("Three URLs", func(t *testing.T) {
		pool := &statusConnectionPool{
			live: []*Connection{
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo1"}},
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo2"}},
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo3"}},
			},
			selector: &roundRobinSelector{curr: -1},
		}

		var expected string
		for i := 0; i < 11; i++ {
			c, err := pool.Next()

			if err != nil {
				t.Errorf("Unexpected error: %s", err)
			}

			switch i % len(pool.live) {
			case 0:
				expected = "http://foo1"
			case 1:
				expected = "http://foo2"
			case 2:
				expected = "http://foo3"
			default:
				t.Fatalf("Unexpected i %% 3: %d", i%3)
			}

			if c.URL.String() != expected {
				t.Errorf("Unexpected URL, want=%s, got=%s", expected, c.URL)
			}
		}
	})

	t.Run("Resurrect dead connection when no live is available", func(t *testing.T) {
		pool := &statusConnectionPool{
			live: []*Connection{},
			dead: []*Connection{
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo1"}, Failures: 3},
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo2"}, Failures: 1},
			},
			selector: &roundRobinSelector{curr: -1},
		}

		c, err := pool.Next()
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}

		if c == nil {
			t.Errorf("Expected connection, got nil: %s", c)
		}

		if c.URL.String() != "http://foo2" {
			t.Errorf("Expected <http://foo2>, got: %s", c.URL.String())
		}

		if c.IsDead {
			t.Errorf("Expected connection to be live, got: %s", c)
		}

		if len(pool.live) != 1 {
			t.Errorf("Expected 1 connection in live list, got: %s", pool.live)
		}

		if len(pool.dead) != 1 {
			t.Errorf("Expected 1 connection in dead list, got: %s", pool.dead)
		}
	})
}

func TestStatusConnectionPoolOnSuccess(t *testing.T) {
	t.Run("Move connection to live list and mark it as healthy", func(t *testing.T) {
		pool := &statusConnectionPool{
			dead: []*Connection{
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo1"}, Failures: 3, IsDead: true},
			},
			selector: &roundRobinSelector{curr: -1},
		}

		conn := pool.dead[0]

		if err := pool.OnSuccess(conn); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if conn.IsDead {
			t.Errorf("Expected the connection to be live; %s", conn)
		}

		if !conn.DeadSince.IsZero() {
			t.Errorf("Unexpected value for DeadSince: %s", conn.DeadSince)
		}

		if len(pool.live) != 1 {
			t.Errorf("Expected 1 live connection, got: %d", len(pool.live))
		}

		if len(pool.dead) != 0 {
			t.Errorf("Expected 0 dead connections, got: %d", len(pool.dead))
		}
	})
}

func TestStatusConnectionPoolOnFailure(t *testing.T) {
	t.Run("Remove connection, mark it, and sort dead connections", func(t *testing.T) {
		pool := &statusConnectionPool{
			live: []*Connection{
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo1"}},
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo2"}},
			},
			dead: []*Connection{
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo3"}, Failures: 0},
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo4"}, Failures: 99},
			},
			selector: &roundRobinSelector{curr: -1},
		}

		conn := pool.live[0]

		if err := pool.OnFailure(conn); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if !conn.IsDead {
			t.Errorf("Expected the connection to be dead; %s", conn)
		}

		if conn.DeadSince.IsZero() {
			t.Errorf("Unexpected value for DeadSince: %s", conn.DeadSince)
		}

		if len(pool.live) != 1 {
			t.Errorf("Expected 1 live connection, got: %d", len(pool.live))
		}

		if len(pool.dead) != 3 {
			t.Errorf("Expected 3 dead connections, got: %d", len(pool.dead))
		}

		expected := []string{
			"http://foo4",
			"http://foo1",
			"http://foo3",
		}

		for i, u := range expected {
			if pool.dead[i].URL.String() != u {
				t.Errorf("Unexpected value for item %d in pool.dead: %s", i, pool.dead[i].URL.String())
			}
		}

		if pool.live[0].URL.String() != "http://foo2" {
			t.Errorf("Expected the first live connection to be http://foo2, got: %s", pool.live[0].URL.String())
		}
	})

	t.Run("Short circuit when the connection is already dead", func(t *testing.T) {
		pool := &statusConnectionPool{
			live: []*Connection{
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo1"}},
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo2"}},
				&Connection{URL: &url.URL{Scheme: "http", Host: "foo3"}},
			},
			selector: &roundRobinSelector{curr: -1},
		}

		conn := pool.live[0]
		conn.IsDead = true

		if err := pool.OnFailure(conn); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if len(pool.dead) != 0 {
			t.Errorf("Expected the dead list to be empty, got: %s", pool.dead)
		}
	})
}

func TestStatusConnectionPoolResurrect(t *testing.T) {
	t.Run("Mark the connection as dead and add/remove it to the lists", func(t *testing.T) {
		pool := &statusConnectionPool{
			live:     []*Connection{},
			dead:     []*Connection{&Connection{URL: &url.URL{Scheme: "http", Host: "foo1"}, IsDead: true}},
			selector: &roundRobinSelector{curr: -1},
		}

		conn := pool.dead[0]

		if err := pool.resurrect(conn, true); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if conn.IsDead {
			t.Errorf("Expected connection to be dead, got: %s", conn)
		}

		if len(pool.dead) != 0 {
			t.Errorf("Expected no dead connections, got: %s", pool.dead)
		}

		if len(pool.live) != 1 {
			t.Errorf("Expected 1 live connection, got: %s", pool.live)
		}
	})

	t.Run("Short circuit removal when the connection is not in the dead list", func(t *testing.T) {
		pool := &statusConnectionPool{
			dead:     []*Connection{&Connection{URL: &url.URL{Scheme: "http", Host: "bar"}, IsDead: true}},
			selector: &roundRobinSelector{curr: -1},
		}

		conn := &Connection{URL: &url.URL{Scheme: "http", Host: "foo1"}, IsDead: true}

		if err := pool.resurrect(conn, true); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if len(pool.live) != 1 {
			t.Errorf("Expected 1 live connection, got: %s", pool.live)
		}

		if len(pool.dead) != 1 {
			t.Errorf("Expected 1 dead connection, got: %s", pool.dead)
		}
	})

	t.Run("Schedule resurrect", func(t *testing.T) {
		defaultResurrectTimeoutInitial = 0
		defer func() { defaultResurrectTimeoutInitial = 60 * time.Second }()

		pool := &statusConnectionPool{
			live: []*Connection{},
			dead: []*Connection{
				&Connection{
					URL:       &url.URL{Scheme: "http", Host: "foo1"},
					Failures:  100,
					IsDead:    true,
					DeadSince: time.Now().UTC(),
				},
			},
			selector: &roundRobinSelector{curr: -1},
		}

		conn := pool.dead[0]
		pool.scheduleResurrect(conn)
		time.Sleep(50 * time.Millisecond)

		pool.Lock()
		defer pool.Unlock()

		if len(pool.live) != 1 {
			t.Errorf("Expected 1 live connection, got: %s", pool.live)
		}
		if len(pool.dead) != 0 {
			t.Errorf("Expected no dead connections, got: %s", pool.dead)
		}
	})
}

func TestConnection(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		conn := &Connection{
			URL:       &url.URL{Scheme: "http", Host: "foo1"},
			Failures:  10,
			IsDead:    true,
			DeadSince: time.Now().UTC(),
		}

		match, err := regexp.MatchString(
			`<http://foo1> dead=true failures=10`,
			conn.String(),
		)

		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if !match {
			t.Errorf("Unexpected output: %s", conn)
		}
	})
}

func TestUpdateConnectionPool(t *testing.T) {
	var initialConnections = []Connection{
		{URL: &url.URL{Scheme: "http", Host: "foo1"}},
		{URL: &url.URL{Scheme: "http", Host: "foo2"}},
		{URL: &url.URL{Scheme: "http", Host: "foo3"}},
	}

	initConnList := func() []*Connection {
		var conns []*Connection
		for i := 0; i < len(initialConnections); i++ {
			conns = append(conns, &initialConnections[i])
		}

		return conns
	}

	t.Run("Update connection pool", func(t *testing.T) {
		pool := &statusConnectionPool{live: initConnList()}

		if len(pool.URLs()) != 3 {
			t.Fatalf("Invalid number of URLs: %d", len(pool.URLs()))
		}

		var updatedConnections = []*Connection{
			{URL: &url.URL{Scheme: "http", Host: "foo1"}},
			{URL: &url.URL{Scheme: "http", Host: "foo2"}},
			{URL: &url.URL{Scheme: "http", Host: "bar1"}},
			{URL: &url.URL{Scheme: "http", Host: "bar2"}},
		}

		_ = pool.Update(updatedConnections)

		if len(pool.URLs()) != 4 {
			t.Fatalf("Invalid number of URLs: %d", len(pool.URLs()))
		}
	})

	t.Run("Update connection removes unknown dead connections", func(t *testing.T) {
		// we start with a bar1 host which shouldn't be there
		pool := &statusConnectionPool{
			live: initConnList(),
			dead: []*Connection{
				{URL: &url.URL{Scheme: "http", Host: "bar1"}},
			},
		}

		if err := pool.Update(initConnList()); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if len(pool.dead) != 0 {
			t.Errorf("Expected no dead connections, got: %s", pool.dead)
		}
	})

	t.Run("Update connection pool with dead connections", func(t *testing.T) {
		pool := &statusConnectionPool{live: initConnList()}

		pool.dead = []*Connection{
			{URL: &url.URL{Scheme: "http", Host: "bar1"}},
		}
		var updatedConnections = []*Connection{
			{URL: &url.URL{Scheme: "http", Host: "foo1"}},
			{URL: &url.URL{Scheme: "http", Host: "foo2"}},
			{URL: &url.URL{Scheme: "http", Host: "bar1"}},
		}

		if err := pool.Update(updatedConnections); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		fmt.Println(pool.live)
		fmt.Println(pool.dead)
	})

	t.Run("Update connection pool with different ports and or path", func(t *testing.T) {
		conns := []Connection{
			{URL: &url.URL{Scheme: "http", Host: "foo1:9200"}},
			{URL: &url.URL{Scheme: "http", Host: "foo1:9205"}},
			{URL: &url.URL{Scheme: "http", Host: "foo1:9200", Path: "/bar1"}},
		}
		pool := &statusConnectionPool{}
		for i := 0; i < len(conns); i++ {
			pool.live = append(pool.live, &conns[i])
		}

		var tmp []*Connection
		for i := 0; i < len(conns); i++ {
			tmp = append(tmp, &conns[i])
		}
		if err := pool.Update(tmp); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if len(pool.live) != len(tmp) {
			t.Errorf("Invalid number of connections: %d", len(pool.live))
		}
	})

	t.Run("Update connection pool lifecycle", func(t *testing.T) {
		// Set up a test connection pool with some initial connections
		cp := &statusConnectionPool{
			live: initConnList(),
		}
		err := cp.Update(initConnList())
		if err != nil {
			t.Errorf("Update() returned an error: %v", err)
		}

		// Test removing a connection that's no longer present
		connections := []*Connection{
			{URL: &url.URL{Scheme: "http", Host: "foo1"}},
			{URL: &url.URL{Scheme: "http", Host: "foo2"}},
		}
		err = cp.Update(connections)
		if err != nil {
			t.Errorf("First Update() returned an error: %v", err)
		}
		if len(cp.live) != 2 {
			t.Errorf("Expected only two live connection after update")
		}

		// foo1 fails
		err = cp.OnFailure(cp.live[0])
		if err != nil {
			t.Errorf("OnFailure() returned an error: %v", err)
		}
		// we update the connexion, nothing should move
		err = cp.Update(connections)
		if err != nil {
			t.Errorf("Second Update() returned an error: %v", err)
		}
		if len(cp.live) != 1 {
			t.Errorf("Expected no connections to be added to lists")
		}

		// Test adding a new connection that's not already present
		connections = append(connections, &Connection{URL: &url.URL{Scheme: "http", Host: "foo12"}})
		err = cp.Update(connections)
		if err != nil {
			t.Errorf("Third Update() returned an error: %v", err)
		}
		if len(cp.live) != 2 {
			t.Errorf("Expected the new connection to be added to live list")
		}

		if err := cp.resurrect(cp.dead[0], false); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		// Test updating with an empty list of connections
		connections = []*Connection{}
		_ = cp.Update(connections)
		if len(cp.live) != 3 {
			t.Errorf("Expected connections to be untouched after empty update")
		}
	})

	t.Run("Update connection pool with discovery", func(t *testing.T) {
		cp := &statusConnectionPool{
			live:     initConnList(),
			selector: &roundRobinSelector{curr: -1},
		}

		connections := []*Connection{
			{URL: &url.URL{Scheme: "http", Host: "foo2"}},
			{URL: &url.URL{Scheme: "http", Host: "foo3"}},
		}

		conn, err := cp.Next()
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if conn.URL.Host != "foo1" {
			t.Errorf("Unexpected host: %s", conn.URL.Host)
		}

		// Update happens between Next and OnFailure
		if err := cp.Update(connections); err != nil {
			t.Errorf("unexpected error: %s", err)
		}

		// conn fails, doesn't exist in live list anymore
		err = cp.OnFailure(conn)
		if err == nil {
			t.Errorf("OnFailure() returned an unexpected error")
		}

		if len(cp.dead) != 0 {
			t.Errorf("OnFailure() should not add unknown live connections to dead list")
		}
	})
}

func TestCloseConnectionPool(t *testing.T) {
	t.Run("CloseConnectionPool", func(t *testing.T) {
		pool := &statusConnectionPool{
			live: []*Connection{
				{URL: &url.URL{Scheme: "http", Host: "foo1"}},
				{URL: &url.URL{Scheme: "http", Host: "foo2"}},
			},
			selector: &roundRobinSelector{curr: -1},
			closeC:   make(chan struct{}),
		}

		err := pool.Close(context.Background())
		if err != nil {
			t.Errorf("Close() returned an error: %v", err)
		}

		err = pool.Close(context.Background())
		if err == nil {
			t.Errorf("Second call to Close() should return an error")
		} else if !strings.Contains(err.Error(), "already closed") {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("CloseConnectionPool isClosed", func(t *testing.T) {
		pool := &statusConnectionPool{
			live: []*Connection{
				{URL: &url.URL{Scheme: "http", Host: "foo1"}},
				{URL: &url.URL{Scheme: "http", Host: "foo2"}},
			},
			selector: &roundRobinSelector{curr: -1},
			closeC:   make(chan struct{}),
		}

		if pool.isClosed() {
			t.Errorf("isClosed() returned true before closing")
		}
		err := pool.Close(context.Background())
		if err != nil {
			t.Errorf("Close() returned an error: %v", err)
		}
		if !pool.isClosed() {
			t.Errorf("isClosed() returned false after close")
		}
	})

	t.Run("CloseConnectionPool abort scheduled resurrect", func(t *testing.T) {
		deadConn := &Connection{URL: &url.URL{Scheme: "http", Host: "foo3"}, IsDead: true, DeadSince: time.Now()}

		pool := &statusConnectionPool{
			live: []*Connection{
				{URL: &url.URL{Scheme: "http", Host: "foo1"}},
				{URL: &url.URL{Scheme: "http", Host: "foo2"}},
			},
			dead: []*Connection{
				deadConn,
			},
			selector: &roundRobinSelector{curr: -1},
			closeC:   make(chan struct{}),
		}

		pool.scheduleResurrect(deadConn)

		err := pool.Close(context.Background())
		if err != nil {
			t.Errorf("Close() returned an error: %v", err)
		}

		if count := len(pool.live); count != 2 {
			t.Errorf("Should have two live connections after Close(), got %d", count)
		}

		if count := len(pool.dead); count != 1 {
			t.Errorf("Should have one dead connection after Close(), got %d", count)
		}
	})

	t.Run("CloseConnectionPool nil context", func(t *testing.T) {
		pool := &statusConnectionPool{
			live: []*Connection{
				{URL: &url.URL{Scheme: "http", Host: "foo1"}},
				{URL: &url.URL{Scheme: "http", Host: "foo2"}},
			},
			selector: &roundRobinSelector{curr: -1},
			closeC:   make(chan struct{}),
		}
		err := pool.Close(nil) //nolint:staticcheck
		if err != nil {
			t.Errorf("Close() returned an error: %v", err)
		}
	})

	t.Run("CloseConnectionPool should timeout", func(t *testing.T) {
		pool := &statusConnectionPool{
			live: []*Connection{
				{URL: &url.URL{Scheme: "http", Host: "foo1"}},
				{URL: &url.URL{Scheme: "http", Host: "foo2"}},
			},
			selector: &roundRobinSelector{curr: -1},
			closeC:   make(chan struct{}),
		}
		// Add to waitgroup that will never be resolved
		pool.resurrectWaitGroup.Add(1)

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()

		err := pool.Close(ctx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Close() did not timeout")
		}
	})

	t.Run("CloseConnectionPool Next() should error if closed", func(t *testing.T) {
		pool := &statusConnectionPool{
			live: []*Connection{
				{URL: &url.URL{Scheme: "http", Host: "foo1"}},
				{URL: &url.URL{Scheme: "http", Host: "foo2"}},
			},
			selector: &roundRobinSelector{curr: -1},
			closeC:   make(chan struct{}),
		}

		err := pool.Close(context.Background())
		if err != nil {
			t.Errorf("Close() returned an error: %v", err)
		}

		_, err = pool.Next()
		if err == nil {
			t.Errorf("Next() returned nil error")
		} else {
			if !strings.Contains(err.Error(), "connection pool is closed") {
				t.Errorf("Next() did not return expected error")
			}
		}
	})
}

func TestNewConnectionPool(t *testing.T) {
	t.Run("Clones the connection slice", func(t *testing.T) {
		conns := []*Connection{
			{URL: &url.URL{Scheme: "http", Host: "foo1"}},
			{URL: &url.URL{Scheme: "http", Host: "foo2"}},
		}
		pool, err := NewConnectionPool(conns, nil)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}

		scp, ok := pool.(*statusConnectionPool)
		if !ok {
			t.Errorf("unexpected type: %T", pool)
		}
		for i := range conns {
			scp.live[i] = nil
		}

		// If conns isn't cloned, the caller will modify the live list
		for i := range conns {
			if conns[i] == nil {
				t.Errorf("expected connection to be cloned")
			}
		}
	})
}
