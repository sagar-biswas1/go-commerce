package cmd

import (
	"go-commerce/global_router"
	productHandlers "go-commerce/product/handlers"
	"log"
	"net/http"
	"time"
)

func Serve(){
	mux := http.NewServeMux()

	mux.HandleFunc("GET /products", productHandlers.GetProducts)
	mux.HandleFunc("POST /products", productHandlers.CreateProduct)
	mux.HandleFunc("GET /products/{id}", productHandlers.GetProduct)
	mux.HandleFunc("PATCH /products/{id}", productHandlers.PatchProduct)
	mux.HandleFunc("DELETE /products/{id}", productHandlers.DeleteProduct)

	server:= &http.Server{
		Addr:         ":8080",
		Handler:      global_router.GlobalRouter(mux),

		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("Server listening on", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
