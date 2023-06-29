package dotnet

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/go-logr/logr"
	"github.com/konveyor/analyzer-lsp/provider"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"gopkg.in/yaml.v2"
)

type dotnetServiceClient struct {
	rpc        jsonrpc2.Conn
	ctx        context.Context
	cancelFunc context.CancelFunc
	cmd        *exec.Cmd
	log        logr.Logger

	config provider.InitConfig
}

var _ provider.ServiceClient = &dotnetServiceClient{}

func (d *dotnetServiceClient) Stop() {
	d.cancelFunc()
	d.cmd.Wait()
}

func (d *dotnetServiceClient) Evaluate(cap string, conditionInfo []byte) (provider.ProviderEvaluateResponse, error) {
	var cond dotnetCondition
	err := yaml.Unmarshal(conditionInfo, &cond)
	if err != nil {
		return provider.ProviderEvaluateResponse{}, fmt.Errorf("unable to get query info")
	}

	query := cond.Referenced
	if query == "" {
		return provider.ProviderEvaluateResponse{}, fmt.Errorf("unable to get query info")
	}

	symbols := d.GetAllSymbols(query)
	incidents := []provider.IncidentContext{}
	for _, s := range symbols {
		if s.Kind == protocol.SymbolKindMethod {
			references := d.GetAllReferences(s)
			for _, ref := range references {
				if strings.Contains(ref.URI.Filename(), d.config.Location) {
					lineNumber := int(ref.Range.Start.Line)
					incidents = append(incidents, provider.IncidentContext{
						FileURI: ref.URI,
						LineNumber: &lineNumber,
						Variables: map[string]interface{}{
							"file": ref.URI.Filename(),
						},
						CodeLocation: &provider.Location{
							StartPosition: provider.Position{Line: float64(lineNumber)},
							EndPosition:   provider.Position{Line: float64(lineNumber)},
						},
					})
				}
			}
		}
	}

	if len(incidents) == 0 {
		return provider.ProviderEvaluateResponse{Matched: false}, nil
	}
	return provider.ProviderEvaluateResponse{
		Matched: true,
		Incidents: incidents,
	}, nil
}

func (d *dotnetServiceClient) GetAllSymbols(query string) []protocol.SymbolInformation {
	wsp := &protocol.WorkspaceSymbolParams{
		Query: query,
	}

	var refs []protocol.SymbolInformation
	_, err := d.rpc.Call(context.TODO(), protocol.MethodWorkspaceSymbol, wsp, &refs)
	if err != nil {
		d.log.Error(err, "failed to get workspace symbols")
	}

	return refs
}

func (d *dotnetServiceClient) GetAllReferences(symbol protocol.SymbolInformation) []protocol.Location {
	params := &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: symbol.Location.URI,
			},
			Position: symbol.Location.Range.Start,
		},
	}

	res := []protocol.Location{}
	_, err := d.rpc.Call(d.ctx, protocol.MethodTextDocumentReferences, params, &res)
	if err != nil {
		d.log.Error(err, "failed to get references")
	}
	return res
}

func (d *dotnetServiceClient) GetDependencies() (map[uri.URI][]*provider.Dep, error) {
	return map[uri.URI][]*provider.Dep{}, nil
}

func (d *dotnetServiceClient) GetDependenciesDAG() (map[uri.URI][]provider.DepDAGItem, error) {
	return map[uri.URI][]provider.DepDAGItem{}, nil
}

// this is a struct for providing the server that lives on the client side
// of the connection to properly respond to requests sent server -> client
type handler struct {
	log *logr.Logger
	ch  chan int
}

func (h *handler) replyHandler(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	method := req.Method()
	// params, _ := req.Params().MarshalJSON()
	switch method {
	case protocol.MethodClientRegisterCapability:
		// d.log.V(2).Info("Got request for client/registerCapability")
		err := reply(ctx, nil, nil)
		// h.ch <- 1
		return err
	case protocol.MethodWorkspaceConfiguration:
		// l.Printf("Got request for workspace configuration %s", params)
		err := reply(ctx, nil, nil)
		// h.ch <- 2
		return err
	case protocol.MethodWindowShowMessage:
		// d.log.V(2).Info("Got request for client/registerCapability")
		// l.Printf("Got request to show a message %s", params)
		var showMessageParams protocol.ShowMessageParams
		if err := json.Unmarshal(req.Params(), &showMessageParams); err != nil {
			return reply(ctx, nil, err)
		}
		err := reply(ctx, nil, nil)
		if strings.HasSuffix(showMessageParams.Message, "project files loaded") {
			// l.Printf("Hey. They told me are ready")
			h.ch <- 3
		}
		return err
	}

	h.log.Info("I don't know what to do with this", req)
	return jsonrpc2.MethodNotFoundHandler(ctx, reply, req)
}
