package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

type Product struct{
	ID int `json:"id"`
	Title string `json:"title"`
	Price float64 `json:"price"`
	ImgUrl string `json:"imageUrl"`
	Description string `json:"description"`
}

var productList = []Product{
	{
		ID:          1,
		Title:       "Wireless Noise-Canceling Headphones",
		Price:       199.99,
		ImgUrl:      "https://example.com/images/headphones.jpg",
		Description: "Over-ear Bluetooth headphones with active noise cancellation and 30-hour battery life.",
	},
	{
		ID:          2,
		Title:       "Ergonomic Mechanical Keyboard",
		Price:       129.50,
		ImgUrl:      "https://example.com/images/keyboard.jpg",
		Description: "Split-layout mechanical keyboard featuring tactile switches and customizable RGB backlighting.",
	},
	{
		ID:          3,
		Title:       "Smart Fitness Watch",
		Price:       89.95,
		ImgUrl:      "https://example.com/images/watch.jpg",
		Description: "Water-resistant smartwatch with continuous heart rate monitoring, built-in GPS, and 7-day battery life.",
	},
}


func handleCors (w http.ResponseWriter,){
        w.Header().Set("Access-Control-Allow-Origin","*")
        w.Header().Set("Access-Control-Allow-Methods","GET, POST, PUT, PATCH, DELETE, OPTIONS")

		w.Header().Set("Access-Control-Allow-Headers","Content-Type, x-api-key")
		w.Header().Set("Content-Type", "application/json")
}

func handlePreFlightReq(w http.ResponseWriter, r *http.Request){
	if r.Method== "OPTIONS"{
			w.WriteHeader(200) 
		}
}

func sendData (w http.ResponseWriter, data interface{}, statusCode int){
        w.WriteHeader(statusCode)
		encoder:=json.NewEncoder(w)
		encoder.Encode(data)
}

func getProducts(w http.ResponseWriter, r *http.Request) {
		handleCors(w)
		handlePreFlightReq(w,r)

		if r.Method!= "GET"{
			http.Error(w, "Method not allowed", 400)
			return 
		}
	sendData(w,productList,200)
		
} 


func createProduct(w http.ResponseWriter, r *http.Request){
		handleCors(w)
		handlePreFlightReq(w,r)
		if r.Method!= "POST"{
			http.Error(w, "Method not allowed", 400)
			return 
		}

	   var newProduct Product

		decoder := json.NewDecoder(r.Body)
		err:=  decoder.Decode(&newProduct)

		if err !=nil{
			http.Error (w,"invalid json",404)
			return 
		}

		newProduct.ID= len(productList) +1

		productList = append(productList, newProduct)
		
		sendData(w,newProduct,201 )

}

func deleteProduct(w http.ResponseWriter, r *http.Request){
    handleCors(w)
	handlePreFlightReq(w,r)
	if r.Method!= "DELETE"{
		http.Error(w, "Method not allowed", 400)
		return 
		}
	idStr:= r.URL.Query().Get("id")
	id, err:= strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid or missing product ID", http.StatusBadRequest) // 400
		return
	}

	for i, p:= range productList{
		if p.ID ==id {
			productList = append(productList[:i], productList[i+1:]...)
			w.WriteHeader(http.StatusOK)
			
			sendData(w,map[string]string{"message": "Product deleted successfully"},http.StatusOK )
			return
		}
	}
	http.Error(w, "Product not found", http.StatusNotFound)

}


func patchProduct(w http.ResponseWriter, r *http.Request) {
	handleCors(w)
	handlePreFlightReq(w,r)
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed) // 405
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid or missing product ID", http.StatusBadRequest) // 400
		return
	}

	var updates struct {
		Title       *string  `json:"title"`
		Price       *float64 `json:"price"`
		ImgUrl      *string  `json:"img_url"`
		Description *string  `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest) // 400
		return
	}

	for i := range productList {
		if productList[i].ID == id {
			if updates.Title != nil {
				productList[i].Title = *updates.Title
			}
			if updates.Price != nil {
				productList[i].Price = *updates.Price
			}
			if updates.ImgUrl != nil {
				productList[i].ImgUrl = *updates.ImgUrl
			}
			if updates.Description != nil {
				productList[i].Description = *updates.Description
			}

			
			sendData(w,productList[i],200)
			return
		}
	}

	http.Error(w, "Product not found", http.StatusNotFound) // 404
}

func main(){
	mux := http.NewServeMux()

	mux.HandleFunc("/product",getProducts)
	mux.HandleFunc("/create-product",createProduct)
	mux.HandleFunc("/delete-product",deleteProduct)
	mux.HandleFunc("/patch-product",patchProduct)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

