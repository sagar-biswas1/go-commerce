package repo

import "sync"

type Product struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Price       float64 `json:"price"`
	ImgUrl      string  `json:"imageUrl"`
	Description string  `json:"description"`
}

type ProductRepo interface {
	All() []Product
	Create(p Product) Product
	ByID(id int) (Product, bool)
	Delete(id int) bool
	Update(id int, apply func(*Product)) (Product, bool)
}

type productRepo struct {
	productList []Product
	nextID      int64
	mu          sync.RWMutex
}

func NewProductRepo() ProductRepo {
	repo := &productRepo{}
	generateInitialProducts(repo)

	return repo
}

func (r *productRepo) All() []Product {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Product, len(r.productList))
	copy(out, r.productList)
	return out
}
func (r *productRepo) Create(p Product) Product {
	r.mu.Lock()
	defer r.mu.Unlock()

	p.ID = int(r.nextID)
	r.nextID++
	r.productList = append(r.productList, p)
	return p
}

func (r *productRepo) ByID(id int) (Product, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.productList {
		if p.ID == id {
			return p, true
		}
	}
	return Product{}, false
}

func (r *productRepo) Delete(id int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, p := range r.productList {
		if p.ID == id {
			r.productList = append(r.productList[:i], r.productList[i+1:]...)
			return true
		}
	}
	return false
}
func (r *productRepo) Update(id int, apply func(*Product)) (Product, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.productList {
		if r.productList[i].ID == id {
			apply(&r.productList[i])
			r.productList[i].ID = id
			return r.productList[i], true
		}
	}
	return Product{}, false
}

func generateInitialProducts(productRepo *productRepo) {
	products := []Product{
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
	productRepo.productList = products
	productRepo.nextID = int64(len(products)) + 1
}
