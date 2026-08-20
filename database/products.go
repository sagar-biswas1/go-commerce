package database

import "sync"

type Product struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Price       float64 `json:"price"`
	ImgUrl      string  `json:"imageUrl"`
	Description string  `json:"description"`
}

// Store holds the products. It is a type rather than a set of package-level
// functions so callers receive one instead of reaching for a global: tests can
// build their own, and two stores never share state by accident.
//
// Every request runs in its own goroutine, so all access goes through mu.
// nextID never reuses an id, even after a delete.
type Store struct {
	mu       sync.RWMutex
	nextID   int
	products []Product
}

// NewProductStore returns a store seeded with the demo catalogue.
func NewProductStore() *Store {
	return &Store{
		nextID: 4,
		products: []Product{
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
		},
	}
}

// All returns a copy of the stored products.
func (s *Store) All() []Product {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Product, len(s.products))
	copy(out, s.products)
	return out
}

// ByID returns the product with the given id, and whether it exists.
func (s *Store) ByID(id int) (Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.products {
		if p.ID == id {
			return p, true
		}
	}
	return Product{}, false
}

// Create stores p under a fresh id and returns the stored copy.
func (s *Store) Create(p Product) Product {
	s.mu.Lock()
	defer s.mu.Unlock()

	p.ID = s.nextID
	s.nextID++
	s.products = append(s.products, p)
	return p
}

// Update applies apply to the stored product with the given id. The id itself
// is restored afterwards so a request body can never change it.
func (s *Store) Update(id int, apply func(*Product)) (Product, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.products {
		if s.products[i].ID == id {
			apply(&s.products[i])
			s.products[i].ID = id
			return s.products[i], true
		}
	}
	return Product{}, false
}

// Delete removes the product with the given id, and reports whether it existed.
func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, p := range s.products {
		if p.ID == id {
			s.products = append(s.products[:i], s.products[i+1:]...)
			return true
		}
	}
	return false
}
