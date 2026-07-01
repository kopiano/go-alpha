package models

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

const faqDir = "/app/assets/faq"
const faqFile = "faq.json"

type Faq struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	Title      string         `gorm:"type:varchar(500);not null" json:"title"`
	Answer     string         `gorm:"type:text;not null" json:"answer"`
	Difficulty string         `gorm:"type:varchar(10);default:easy;not null" json:"difficulty"`
	Category   string         `gorm:"type:varchar(100);default:'';not null" json:"category"`
}

func (Faq) GetAll() ([]Faq, error) {
	var faqs []Faq
	err := DB.Order("id asc").Find(&faqs).Error
	return faqs, err
}

func (f *Faq) Add() error {
	return DB.Create(f).Error
}

func (Faq) GetByID(id uint) (*Faq, error) {
	var faq Faq
	if err := DB.First(&faq, id).Error; err != nil {
		return nil, err
	}
	return &faq, nil
}

func (Faq) DeleteByID(id uint) error {
	return DB.Delete(&Faq{}, id).Error
}

// SeedFaqs inserts initial FAQ data if the table is empty
func SeedFaqs() {
	var count int64
	DB.Model(&Faq{}).Count(&count)
	if count > 0 {
		return
	}

	seeds := []Faq{
		{
			Title:      "What is Go and why was it created?",
			Answer:     "Go (Golang) is a statically typed, compiled programming language designed at Google by Robert Griesemer, Rob Pike, and Ken Thompson.\n\nIt was created to address the challenges of large-scale software development at Google:\n- Slow compilation speeds\n- Cumbersome dependency management\n- The complexity of C++ and Java\n\nGo combines the performance of a compiled language with the simplicity of a scripting language, featuring goroutines for lightweight concurrency, a rich standard library, and built-in tooling for formatting, testing, and dependency management.",
			Difficulty: "easy",
			Category:   "Backend",
		},
		{
			Title:      "How does goroutine scheduling work?",
			Answer:     "Goroutines are lightweight threads managed by the Go runtime (not the OS). The runtime uses an M:N scheduler, multiplexing M goroutines onto N OS threads.\n\nIt employs a work-stealing algorithm where idle processors steal work from busy ones, and supports cooperative preemption.\n\nGo's scheduler uses three key abstractions:\n- G (goroutine)\n- M (machine / OS thread)\n- P (processor / logical CPU)\n\nThe number of P's is set by GOMAXPROCS (defaults to the number of CPU cores). Goroutines are extremely cheap — you can spawn hundreds of thousands with minimal overhead compared to OS threads.",
			Difficulty: "medium",
			Category:   "Backend",
		},
		{
			Title:      "What is the difference between a pointer receiver and a value receiver?",
			Answer:     "A value receiver receives a copy of the struct, so modifications to the receiver won't affect the original value. A pointer receiver receives a pointer to the original struct, so modifications are reflected in the caller.\n\nUse pointer receivers when:\n1. The method needs to modify the receiver\n2. The struct is large and copying would be expensive\n3. For consistency if other methods on the same type use pointer receivers\n\nUse value receivers for small, immutable types where copying is inexpensive and you want to guarantee no mutation.",
			Difficulty: "medium",
			Category:   "Backend",
		},
		{
			Title:      "When should I use channels vs mutexes?",
			Answer:     "Use channels when you need to communicate between goroutines — passing data ownership, signaling completion, or coordinating work.\n\nUse mutexes (sync.Mutex, sync.RWMutex) when you need to protect shared state accessed by multiple goroutines without passing ownership.\n\nThe Go proverb \"Do not communicate by sharing memory; instead, share memory by communicating\" suggests channels as the default, but mutexes are often simpler and more performant for caches, counters, and simple shared state.\n\nIn practice:\n- Channels excel at orchestrating pipelines and work distribution\n- Mutexes excel at guarding data structures",
			Difficulty: "hard",
			Category:   "Backend",
		},
		{
			Title:      "How does Go handle error management?",
			Answer:     "Go uses explicit error handling rather than exceptions. Functions return errors as a second return value, and callers check them immediately.\n\nCommon patterns:\n1. if err != nil { return err } — the most basic propagation\n2. Wrapping errors with fmt.Errorf(\"...: %w\", err) to add context while preserving the original error for errors.Is / errors.As\n3. Defining custom error types\n4. Using defer with named return values for cleanup\n5. Handling errors at the highest appropriate level rather than logging at every layer\n\nGo 1.13+ introduced errors.Is() and errors.As() for error chain inspection.\nGo 1.20+ added errors.Join() for combining multiple errors.",
			Difficulty: "easy",
			Category:   "Backend",
		},
		{
			Title:      "What is the sync.Pool and when should I use it?",
			Answer:     "sync.Pool is a concurrent-safe object pool that caches temporary objects for reuse, reducing GC pressure.\n\nIt's ideal for short-lived allocations that are created frequently — byte buffers, temporary structs, or worker objects.\n\nKey behaviors:\n- Objects in the pool can be garbage collected between GC cycles (so never rely on pool content)\n- Get() returns nil if the pool is empty\n- Always call Put() to return objects after use\n\nUse sync.Pool when:\n1. You allocate the same type frequently in hot paths\n2. Each object is independent (no cross-object state)\n3. You're okay with objects being occasionally recreated\n\nAvoid for long-lived objects or when pooling adds more complexity than the GC savings.",
			Difficulty: "hard",
			Category:   "Backend",
		},
	}

	for i := range seeds {
		DB.Create(&seeds[i])
	}
}

// GetAllFromFile reads FAQ data from assets/faq/faq.json
func GetAllFromFile() ([]Faq, error) {
	path := filepath.Join(faqDir, faqFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var items []struct {
		ID         uint   `json:"id"`
		Title      string `json:"title"`
		Answer     string `json:"answer"`
		Difficulty string `json:"difficulty"`
		Category   string `json:"category"`
		CreatedAt  string `json:"created_at"`
	}
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}

	faqs := make([]Faq, 0, len(items))
	for _, item := range items {
		faqs = append(faqs, Faq{
			ID:         item.ID,
			Title:      item.Title,
			Answer:     item.Answer,
			Difficulty: item.Difficulty,
			Category:   item.Category,
		})
	}
	return faqs, nil
}

// SyncFaqToFile writes all DB FAQ records to assets/faq/faq.json
func SyncFaqToFile() error {
	faqs, err := Faq{}.GetAll()
	if err != nil {
		return err
	}

	type item struct {
		ID         uint   `json:"id"`
		Title      string `json:"title"`
		Answer     string `json:"answer"`
		Difficulty string `json:"difficulty"`
		Category   string `json:"category"`
		CreatedAt  string `json:"created_at"`
	}

	items := make([]item, 0, len(faqs))
	for _, f := range faqs {
		items = append(items, item{
			ID:         f.ID,
			Title:      f.Title,
			Answer:     f.Answer,
			Difficulty: f.Difficulty,
			Category:   f.Category,
			CreatedAt:  f.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(faqDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(faqDir, faqFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	slog.Info("Faq.SyncToFile: done", "count", len(items))
	return nil
}
