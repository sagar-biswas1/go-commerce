package product

import "sync"

type Product struct{
	ID int `json:"id"`
	Title string `json:"title"`
	Price float64 `json:"price"`
	ImgUrl string `json:"imageUrl"`
	Description string `json:"description"`
}

// Every request runs in its own goroutine, so all access to productList goes
// through mu. nextID never reuses an id, even after a delete.
var (
	mu sync.RWMutex
	nextID = 4
	productList = []Product{
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
)

// All returns a copy of the stored products.
func All() []Product {
	mu.RLock()
	defer mu.RUnlock()

	out:= make([]Product, len(productList))
	copy(out, productList)
	return out
}

// ByID returns the product with the given id, and whether it exists.
func ByID(id int) (Product, bool) {
	mu.RLock()
	defer mu.RUnlock()

	for _, p:= range productList {
		if p.ID == id {
			return p, true
		}
	}
	return Product{}, false
}

// Create stores p under a fresh id and returns the stored copy.
func Create(p Product) Product {
	mu.Lock()
	defer mu.Unlock()

	p.ID = nextID
	nextID++
	productList = append(productList, p)
	return p
}

// Update applies apply to the stored product with the given id. The id itself
// is restored afterwards so a request body can never change it.
func Update(id int, apply func(*Product)) (Product, bool) {
	mu.Lock()
	defer mu.Unlock()

	for i:= range productList {
		if productList[i].ID == id {
			apply(&productList[i])
			productList[i].ID = id
			return productList[i], true
		}
	}
	return Product{}, false
}

// Delete removes the product with the given id, and reports whether it existed.
func Delete(id int) bool {
	mu.Lock()
	defer mu.Unlock()

	for i, p:= range productList {
		if p.ID == id {
			productList = append(productList[:i], productList[i+1:]...)
			return true
		}
	}
	return false
}
