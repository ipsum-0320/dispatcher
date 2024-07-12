package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
)

func Serve(ctx context.Context, host string, port string) error {
	router := NewRouter()
	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", host, port),
		Handler: router,
	}
	err := httpServer.ListenAndServe()
	if err != nil {
		log.Printf("Failed to start server: %v\n", err)
		return err
	}

	go func() {
		<-ctx.Done()
		err := httpServer.Shutdown(context.Background())
		if err != nil {
			log.Printf("Shutting down server failed: %v", err)
		}
	}()

	return nil
}
