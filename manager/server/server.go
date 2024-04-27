package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

func Serve(ctx context.Context, host string, port int64) error {
	router := NewRouter()
	httpServer := &http.Server{
		Addr:    host + ":" + strconv.FormatInt(port, 10),
		Handler: router,
	}
	err := httpServer.ListenAndServe()
	if err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		return err
	}

	go func() {
		select {
		case <-ctx.Done():
			err := httpServer.Shutdown(context.Background())
			fmt.Printf("Shutting down server failed: %v", err)
		}
	}()

	return nil
}
