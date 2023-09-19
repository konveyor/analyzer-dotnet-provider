package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

var l = log.New(os.Stdout, "", 0)

type multiplex struct {
	io.Reader
	io.Writer
}

func (m *multiplex) Close() error {
	return nil
}

type langHandler struct {
	c chan int
}

func (h *langHandler) replyHandler(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	method := req.Method()
	switch method {
	case protocol.MethodClientRegisterCapability:
		err := reply(ctx, nil, nil)
		h.c <- 1
		return err
	case protocol.MethodWindowShowMessage:
		var showMessageParams protocol.ShowMessageParams
		if err := json.Unmarshal(req.Params(), &showMessageParams); err != nil {
			return reply(ctx, nil, err)
		}
		err := reply(ctx, nil, nil)
		if( strings.HasSuffix(showMessageParams.Message, "project files loaded") ) {
			h.c <- 3
		}
		return err
	case protocol.MethodWorkspaceConfiguration:
		err := reply(ctx, nil, nil)
		h.c <- 2
		return err
	}

	return jsonrpc2.MethodNotFoundHandler(ctx, reply, req)
}

func main() {
	ctx := context.Background()

	// srv, err := os.Create("/tmp/server")
	// if err != nil {
	// 	fmt.Printf("error: %v", err)
	// 	return
	// }
	// defer srv.Close()

	// need a log file location for our client
	// client, err := os.Create("/tmp/client")
	// if err != nil {
	// 	fmt.Printf("error: %v", err)
	// 	return
	// }
	// defer client.Close()

	var langServer *exec.Cmd = exec.CommandContext(ctx, "dotnet", []string{"/opt/app-root/omnisharp/OmniSharp.dll", "--languageserver", "--source /opt/app-root/src"}...)
	// var langServer *exec.Cmd = exec.CommandContext(ctx, "csharp-ls")

	// make it so we catch what is put on stdin/stdout for debug
	stdin, err := langServer.StdinPipe()
	if err != nil {
		l.Fatal(err)
	}
	clientWriter := io.MultiWriter(os.Stdout, stdin)

	stdout, err := langServer.StdoutPipe()
	if err != nil {
		l.Fatal(err)
	}
	clientReader := io.TeeReader(stdout, os.Stdout)

	// track where we started/start the language server
	langServer.Dir = "/opt/app-root/src"
	// wd, _ := os.Getwd()
	// l.Printf("Starting language server: '%v' in directory '%v'", langServer.String(), wd)
	go func() {
		err := langServer.Run()
		if err != nil {
			l.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)
	l.Println("Language server started")

	serverChannel := make(chan int)
	h := &langHandler{c: serverChannel}

	// Start the jsonrpc2 connection
	conn := jsonrpc2.NewConn(jsonrpc2.NewStream(&multiplex{Reader: clientReader, Writer: clientWriter}))
	handler := jsonrpc2.ReplyHandler(h.replyHandler)
	handlerSrv := jsonrpc2.HandlerServer(handler)
	l.Printf("Starting to jsonrpc2 server (for client)")
	go func() {
		err := handlerSrv.ServeStream(context.Background(), conn)
		if err != nil {
			l.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)
	l.Println("jsonrpc2 server (for client) started")

	params := &protocol.InitializeParams{
		RootURI: uri.File("/opt/app-root/src"),
		Capabilities: protocol.ClientCapabilities{
			TextDocument: &protocol.TextDocumentClientCapabilities{
				DocumentSymbol: &protocol.DocumentSymbolClientCapabilities{
					HierarchicalDocumentSymbolSupport: true,
				},
			},
			Workspace: &protocol.WorkspaceClientCapabilities{
				DidChangeWatchedFiles: &protocol.DidChangeWatchedFilesWorkspaceClientCapabilities{
					DynamicRegistration: false,
				},
				// WorkspaceFolders: true,
			},
		},
		// WorkspaceFolders: []protocol.WorkspaceFolder{
		// 	protocol.WorkspaceFolder{
		// 		URI: "/opt/app-root/src",
		// 		Name: "workspace",
		// 	},
		// },
	}

	l.Printf("Initalizing language server")
	var result protocol.InitializeResult
	for {
		_, err := conn.Call(ctx, protocol.MethodInitialize, params, &result)
		if err != nil {
			l.Printf("waiting to initialize...failed: %v", err)
			continue
		}
		break
	}
	l.Printf("Server responded to initialize request")

	l.Printf("Sending initialized notificationn")
	if err := conn.Notify(ctx, protocol.MethodInitialized, &protocol.InitializedParams{}); err != nil {
		fmt.Printf("initialized failed: %v\n", err)
		return
	}
	l.Printf("Language server initialized")

	// l.Printf("Waiting until we handle the client/registerCapability request")
	// <-serverChannel
	// l.Printf("client/registerCapability request received")
	//
	// l.Printf("Waiting until we handle the client/workspaceConfiguration request")
	// <-serverChannel
	// l.Printf("client/workspaceConfiguration request received")
	//
	// l.Printf("Waiting until they say they are ready")
	// <-serverChannel
	// l.Printf("They say they are ready")

	l.Printf("Attempting a query")
		// Query: "HttpNotFound()",
		// Query: "",
	wsParams := &protocol.WorkspaceSymbolParams{
		Query: "HttpNotFound",
	}
	var wsQueryResult []protocol.SymbolInformation
	_, err = conn.Call(ctx, protocol.MethodWorkspaceSymbol, wsParams, &wsQueryResult)
	if err != nil {
		l.Fatal(err)
	}
	l.Printf("Result: %v", wsQueryResult)
}
