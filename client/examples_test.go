/*
 * Copyright (C) 2017 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package client_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"

	"github.com/dgraph-io/dgraph/client"
	"github.com/dgraph-io/dgraph/x"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"
)

func Node(val string, c *client.Dgraph) string {
	if uid, err := strconv.ParseUint(val, 0, 64); err == nil {
		return c.NodeUid(uid).String()
	}
	if strings.HasPrefix(val, "_:") {
		n, err := c.NodeBlank(val[2:])
		if err != nil {
			log.Fatalf("Error while converting to node: %v", err)
		}
		return n.String()
	}
	n, err := c.NodeXid(val, false)
	if err != nil {
		log.Fatalf("Error while converting to node: %v", err)
	}
	return n.String()
}

func ExampleReq_Set() {
	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	x.Checkf(err, "While trying to dial gRPC")
	defer conn.Close()

	clientDir, err := ioutil.TempDir("", "client_")
	x.Check(err)
	dgraphClient := client.NewDgraphClient([]*grpc.ClientConn{conn}, client.DefaultOptions, clientDir)

	// Create new request
	req := client.Req{}

	// Create a node for person1 (the blank node label "person1" exists
	// client-side so the mutation can correctly link nodes.  It is not
	// persisted in the server)
	person1, err := dgraphClient.NodeBlank("person1")
	if err != nil {
		log.Fatal(err)
	}

	// Add edges for name and salary to person1
	e := person1.Edge("name")
	e.SetValueString("Steven Spielberg")
	req.Set(e)

	// If the old variable was written over or outof scope we can lookup person1 again,
	// the string->node mapping is remembered by the client for this session.
	p, err := dgraphClient.NodeBlank("person1")
	e = p.Edge("salary")
	e.SetValueFloat(13333.6161)
	req.Set(e)

	resp, err := dgraphClient.Run(context.Background(), &req)
	if err != nil {
		log.Fatalf("Error in getting response from server, %s", err)
	}
	fmt.Printf("%+v\n", proto.MarshalTextString(resp))
}

func ExampleDgraph_BatchSet() {
	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	x.Checkf(err, "While trying to dial gRPC")
	defer conn.Close()

	bmOpts := client.BatchMutationOptions{
		Size:          1000,
		Pending:       100,
		PrintCounters: false,
	}
	clientDir, err := ioutil.TempDir("", "client_")
	x.Check(err)
	dgraphClient := client.NewDgraphClient([]*grpc.ClientConn{conn}, bmOpts, clientDir)

	// Create a node for person1 (the blank node label "person1" exists
	// client-side so the mutation can correctly link nodes.  It is not
	// persisted in the server)
	person1, err := dgraphClient.NodeBlank("person1")
	if err != nil {
		log.Fatal(err)
	}

	// Add edges for name and salary to the batch mutation
	e := person1.Edge("name")
	e.SetValueString("Steven Spielberg")
	dgraphClient.BatchSet(e)
	e = person1.Edge("salary")
	e.SetValueFloat(13333.6161)
	dgraphClient.BatchSet(e)

	dgraphClient.BatchFlush() // Must be called to flush buffers after all mutations are added.
}

func ExampleEdge_AddFacet() {
	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	x.Checkf(err, "While trying to dial gRPC")
	defer conn.Close()

	clientDir, err := ioutil.TempDir("", "client_")
	x.Check(err)
	dgraphClient := client.NewDgraphClient([]*grpc.ClientConn{conn}, client.DefaultOptions, clientDir)

	req := client.Req{}

	// Create a node for person1 add an edge for name.
	person1, err := dgraphClient.NodeXid("person1", false)
	if err != nil {
		log.Fatal(err)
	}
	e := person1.Edge("name")
	e.SetValueString("Steven Spielberg")

	// Add facets since and alias to the edge.
	e.AddFacet("since", "2006-01-02T15:04:05")
	e.AddFacet("alias", `"Steve"`)

	req.Set(e)

	person2, err := dgraphClient.NodeXid("person2", false)
	if err != nil {
		log.Fatal(err)
	}
	e = person2.Edge("name")
	e.SetValueString("William Jones")
	req.Set(e)

	e = person1.ConnectTo("friend", person2)

	// Facet on a node-node edge.
	e.AddFacet("close", "true")
	req.Set(e)

	req.AddSchemaFromString(`
name: string @index(exact) .
`)

	req.SetQuery(`{
		me(func: eq(name,"Steven Spielberg")) {
			name @facets
			friend @facets {
				name
			}
		}
	}`)

	// Run the request in the Dgraph server.  The mutations are added, then
	// the query is exectuted.
	resp, err := dgraphClient.Run(context.Background(), &req)
	if err != nil {
		log.Fatalf("Error in getting response from server, %s", err)
	}
	fmt.Printf("%+v\n", proto.MarshalTextString(resp))
}

func ExampleReq_AddSchemaFromString() {
	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	x.Checkf(err, "While trying to dial gRPC")
	defer conn.Close()

	bmOpts := client.BatchMutationOptions{
		Size:          1000,
		Pending:       100,
		PrintCounters: false,
	}
	clientDir, err := ioutil.TempDir("", "client_")
	x.Check(err)
	dgraphClient := client.NewDgraphClient([]*grpc.ClientConn{conn}, bmOpts, clientDir)

	req := client.Req{}

	// Add a schema mutation to the request
	err = req.AddSchemaFromString(`
name: string @index(term) .
release_date: dateTime @index .
`)
	if err != nil {
		log.Fatalf("Error setting schema, %s", err)
	}

	// Query the changed schema
	req.SetQuery(`schema {}`)
	resp, err := dgraphClient.Run(context.Background(), &req)
	if err != nil {
		log.Fatalf("Error in getting response from server, %s", err)
	}
	fmt.Printf("%+v\n", proto.MarshalTextString(resp))
}

func ExampleReq_SetQuery() {
	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	x.Checkf(err, "While trying to dial gRPC")
	defer conn.Close()

	bmOpts := client.BatchMutationOptions{
		Size:          1000,
		Pending:       100,
		PrintCounters: false,
	}
	clientDir, err := ioutil.TempDir("", "client_")
	x.Check(err)
	dgraphClient := client.NewDgraphClient([]*grpc.ClientConn{conn}, bmOpts, clientDir)

	req := client.Req{}
	alice, err := dgraphClient.NodeXid("alice", false)
	if err != nil {
		log.Fatal(err)
	}
	e := alice.Edge("name")
	e.SetValueString("Alice")
	req.Set(e)

	e = alice.Edge("falls.in")
	e.SetValueString("Rabbit hole")
	req.Set(e)

	req.AddSchemaFromString(`name: string @index(exact) .`)
	if err != nil {
		log.Fatalf("Error setting schema, %s", err)
	}

	req.SetQuery(`{
		me(func: eq(name, "Alice")) {
			name
			falls.in
		}
	}`)
	resp, err := dgraphClient.Run(context.Background(), &req)
	if err != nil {
		log.Fatalf("Error in getting response from server, %s", err)
	}
	fmt.Printf("%+v\n", proto.MarshalTextString(resp))
}


func ExampleReq_SetQueryWithVariables() {
	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	x.Checkf(err, "While trying to dial gRPC")
	defer conn.Close()

	bmOpts := client.BatchMutationOptions{
		Size:          1000,
		Pending:       100,
		PrintCounters: false,
	}
	clientDir, err := ioutil.TempDir("", "client_")
	x.Check(err)
	dgraphClient := client.NewDgraphClient([]*grpc.ClientConn{conn}, bmOpts, clientDir)

	req := client.Req{}

	alice, err := dgraphClient.NodeXid("alice", false)
	if err != nil {
		log.Fatal(err)
	}
	e := alice.Edge("name")
	e.SetValueString("Alice")
	req.Set(e)

	e = alice.Edge("falls.in")
	e.SetValueString("Rabbit hole")
	req.Set(e)

	req.AddSchemaFromString(`name: string @index(exact) .`)

	variables := make(map[string]string)
	variables["$a"] = "Alice"
	req.SetQueryWithVariables(`{
		me(func: eq(name, $a)) {
			name
			falls.in
		}
	}`, variables)


	resp, err := dgraphClient.Run(context.Background(), &req)
	fmt.Printf("%+v\n", proto.MarshalTextString(resp))
}



func ExampleDgraph_NodeUidVar() {
	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	x.Checkf(err, "While trying to dial gRPC")
	defer conn.Close()

	bmOpts := client.BatchMutationOptions{
		Size:          1000,
		Pending:       100,
		PrintCounters: false,
	}
	clientDir, err := ioutil.TempDir("", "client_")
	x.Check(err)
	dgraphClient := client.NewDgraphClient([]*grpc.ClientConn{conn}, bmOpts, clientDir)

	req := client.Req{}

	// Add some data
	alice, err := dgraphClient.NodeXid("alice", false)
	if err != nil {
		log.Fatal(err)
	}
	e := alice.Edge("name")
	e.SetValueString("Alice")
	req.Set(e)

	req.AddSchemaFromString(`name: string @index(exact) .`)

	resp, err := dgraphClient.Run(context.Background(), &req)

	// New request
	req = client.Req{}

	// Now issue a query and mutation using client interface

    req.SetQuery(`{
    a as var(func: eq(name, "Alice"))
    me(func: uid(a)) {
        name
    }
}`)

	// Get a node for the variable a in the query above.
    n, _ := dgraphClient.NodeUidVar("a")
    e = n.Edge("falls.in")
	e.SetValueString("Rabbit hole")
	req.Set(e)

    resp, err = dgraphClient.Run(context.Background(), &req)
	if err != nil {
		log.Fatalf("Error in getting response from server, %s", err)
	}
	fmt.Printf("%+v\n", proto.MarshalTextString(resp))


	// This is equivalent to the single query and mutation 
	//
	// {
    //		a as var(func: eq(name, "Alice"))
    //		me(func: uid(a)) {
	//			name
    //		}
	// }
	// mutation { set {
	//		var(a) <falls.in> "Rabbit hole" .
	// }} 
	//
	// It's often easier to construct such things with client functions that
	// by manipulating raw strings.
}


func ExampleEdge_SetValueBytes() {
	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	x.Checkf(err, "While trying to dial gRPC")
	defer conn.Close()

	clientDir, err := ioutil.TempDir("", "client_")
	x.Check(err)
	dgraphClient := client.NewDgraphClient([]*grpc.ClientConn{conn}, client.DefaultOptions, clientDir)

	req := client.Req{}

	alice, err := dgraphClient.NodeBlank("alice")
	if err != nil {
		log.Fatal(err)
	}
	e := alice.Edge("name")
	e.SetValueString("Alice")
	req.Set(e)

	e = alice.Edge("somestoredbytes")
	err = e.SetValueBytes([]byte(`\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98`))
	x.Check(err)
	req.Set(e)

	req.AddSchemaFromString(`name: string @index(exact) .`)

	req.SetQuery(`{
	q(func: eq(name, "Alice")) {
		name
		somestoredbytes
	}
}`)

	resp, err := dgraphClient.Run(context.Background(), &req)
	if err != nil {
		log.Fatalf("Error in getting response from server, %s", err)
	}
	fmt.Printf("%+v\n", proto.MarshalTextString(resp))
}


func ExampleUnmarshal() {
	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	x.Checkf(err, "While trying to dial gRPC")
	defer conn.Close()

	clientDir, err := ioutil.TempDir("", "client_")
	x.Check(err)
	dgraphClient := client.NewDgraphClient([]*grpc.ClientConn{conn}, client.DefaultOptions, clientDir)

	req := client.Req{}

	// A mutation as a string, see ExampleReq_NodeUidVar, ExampleReq_SetQuery,
	// etc for examples of mutations using client functions.
	req.SetQuery(`
mutation {
	schema {
		name: string @index .
	}
	set {
		_:person1 <name> "Alex" .
		_:person2 <name> "Beatie" .
		_:person3 <name> "Chris" .

		_:person1 <friend> _:person2 .
		_:person1 <friend> _:person3 .
	}
}
{
	friends(func: eq(name, "Alex")) {
		name
		friend {
			name
		}
	}	
}`)	

	// Run the request in the Dgraph server.  The mutations are added, then
	// the query is exectuted.
	resp, err := dgraphClient.Run(context.Background(), &req)
	if err != nil {
		log.Fatalf("Error in getting response from server, %s", err)
	}
	
	// Unmarshal the response into a custom struct

	// A type representing information in the graph.
	type person struct {
		Name string `dgraph:"name"`
		Friends []person `dgraph:"friend"`
	}

	// A helper type matching the query root.
	type friends struct {
		Root person `dgraph:"friends"`
	} 

	var f friends
	err = client.Unmarshal(resp.N, &f)
	if err != nil {
		log.Fatal("Couldn't unmarshal response : ", err)
	}

	fmt.Println("Name : ", f.Root.Name)
	fmt.Print("Friends : ")
	for _, p := range f.Root.Friends {
		fmt.Print(p.Name, " ")
	} 
	fmt.Println()

}
