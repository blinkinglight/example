package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/blinkinglight/example/templates"
	"github.com/delaneyj/toolbelt/embeddednats"
	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go"
	"github.com/starfederation/datastar/sdk/go/datastar"
)

type Command struct {
	Action string `json:"action"`
	Input  string `json:"input"`
}

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	ctx := context.Background()

	ns, err := embeddednats.New(ctx, embeddednats.WithDirectory("./data/nats"), embeddednats.WithShouldClearData(true))
	if err != nil {
		panic(err)
	}

	nc, err := ns.Client()
	if err != nil {
		panic(err)
	}

	sendCommand := func(correlationID, action, input string) error {
		cmd := &Command{
			Action: action,
			Input:  input,
		}
		data, err := json.Marshal(cmd)
		if err != nil {
			slog.Error("Failed to marshal command", "error", err)
			return err
		}
		if correlationID != "" {
			if err := nc.Publish(fmt.Sprintf("data.pipe.%s", correlationID), data); err != nil {
				slog.Error("Failed to publish command", "error", err)
				return err
			}
			return nil
		}
		if err := nc.Publish("data.pipe", data); err != nil {
			slog.Error("Failed to publish command", "error", err)
			return err
		}
		return nil
	}

	router := chi.NewMux()

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		templates.PageHome(templates.Page{}).Render(r.Context(), w)
	})

	router.Get("/pipe", func(w http.ResponseWriter, r *http.Request) {
		var signals struct {
			CorrelationID string `json:"correlationID"`
		}
		if err := datastar.ReadSignals(r, &signals); err != nil {
			http.Error(w, "Failed to read signals", http.StatusBadRequest)
			return
		}

		w.WriteHeader(200)
		sse := datastar.NewSSE(w, r)

		var pipe = make(chan *Command)
		sub, err := nc.Subscribe("data.pipe", func(msg *nats.Msg) {
			var cmd *Command
			json.Unmarshal(msg.Data, &cmd)
			pipe <- cmd
		})
		if err != nil {
			sse.PatchElementTempl(templates.ToastError("Failed to subscribe to data.pipe"))
			return
		}

		subPersonal, err := nc.Subscribe("data.pipe."+signals.CorrelationID, func(msg *nats.Msg) {
			var cmd *Command
			json.Unmarshal(msg.Data, &cmd)
			pipe <- cmd
		})
		if err != nil {
			sse.PatchElementTempl(templates.ToastError("Failed to subscribe to data.pipe"))
			return
		}

		defer sub.Unsubscribe()
		defer subPersonal.Unsubscribe()
		var state = templates.Page{}
		for {
			select {
			case <-sse.Context().Done():
				return
			case cmd := <-pipe:
				switch cmd.Action {
				case "update-and-render-list":
					state.List = append(state.List, cmd.Input)
					sse.PatchElementTempl(templates.Partial(state))
				case "show-error":
					sse.PatchElementTempl(templates.ToastError(cmd.Input))
				}

			}
		}
	})

	router.Post("/", func(w http.ResponseWriter, r *http.Request) {
		var signals struct {
			Input         string `json:"input"`
			CorrelationID string `json:"correlationID"`
		}
		if err := datastar.ReadSignals(r, &signals); err != nil {
			return
		}
		datastar.NewSSE(w, r)

		if signals.Input == "" {
			sendCommand(signals.CorrelationID, "show-error", "Input cannot be empty")
			return
		}
		sendCommand("", "update-and-render-list", signals.Input)
	})

	router.Post("/error", func(w http.ResponseWriter, r *http.Request) {
		var signals struct {
			CorrelationID string `json:"correlationID"`
		}
		if err := datastar.ReadSignals(r, &signals); err != nil {
			return
		}
		datastar.NewSSE(w, r)
		sendCommand(signals.CorrelationID, "show-error", "This is a test error message")
	})

	slog.Info("Starting server on :9999")
	if err := http.ListenAndServe(":9999", router); err != nil {
		slog.Error("Failed to start server", "error", err)
		return
	}
	slog.Info("Server stopped")
}
