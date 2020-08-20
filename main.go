package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const maxLimit = 50

type Book struct {
	BookInformation BookInformation `json:"book_information"`
}

type BookInformation struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Authors     []string `json:"authors"`
	Categories  []string `json:"categories"`
	Pages       int      `json:"pageCount"`
	Images      struct {
		Small  string `json:"smallThumbnail"`
		Normal string `json:"thumbnail"`
	} `json:"imageLinks"`
}

func main() {
	router := mux.NewRouter()

	router.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`pong`))
	})

	router.HandleFunc("/books", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if query == "" {
			http.Error(w, "query is required", http.StatusBadRequest)
			return
		}

		limit := r.URL.Query().Get("limit")
		if limit == "" {
			http.Error(w, "limit is required", http.StatusBadRequest)
			return
		}

		l, err := strconv.Atoi(limit)
		if err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}

		if l < 0 {
			http.Error(w, "limit cant be a negative number", http.StatusBadRequest)
			return
		}

		books, err := getBooks(query, l)
		if err != nil {
			http.Error(w, fmt.Sprintf("getting books: %v", err), http.StatusInternalServerError)
			return
		}

		response := struct {
			Query string `json:"query"`
			Total int    `json:"total"`
			Books []Book `json:"books"`
		}{
			Query: query,
			Total: len(books),
			Books: books,
		}

		v, err := json.Marshal(&response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(v)
	}).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := http.ListenAndServe(":" + port, router); err != nil {
		panic(err)
	}
}

func getBooks(query string, limit int) ([]Book, error) {
	var books []Book

	q := url.QueryEscape(query)

	if limit > maxLimit {
		limit = maxLimit
	}

	if limit == 0 {
		return []Book{}, nil
	}

	for len(books) < limit {
		resp, err := http.Get(fmt.Sprintf("https://www.googleapis.com/books/v1/volumes?q=%s", q))
		if err != nil {
			return []Book{}, fmt.Errorf("making GET request: %v", err)
		}

		var response struct {
			Items []struct {
				BookInformation BookInformation `json:"volumeInfo"`
			} `json:"items"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return []Book{}, err
		}

		if len(response.Items) == 0 && len(books) == 0 {
			return []Book{}, nil
		} else if len(response.Items) == 0 {
			return books, nil
		}

		for _, item := range response.Items {
			if len(books) >= limit {
				break
			}

			if hasEmptyInformation(item.BookInformation) {
				continue
			}

			books = append(books, Book{
				BookInformation: item.BookInformation,
			})
		}
	}

	return books, nil
}

func hasEmptyInformation(bookInformation BookInformation) bool {
	return bookInformation.Pages == 0 ||
		bookInformation.Description == "" ||
		len(bookInformation.Authors) == 0 ||
		bookInformation.Images.Normal == "" ||
		bookInformation.Images.Small == "" ||
		bookInformation.Title == "" ||
		len(bookInformation.Categories) == 0
}
