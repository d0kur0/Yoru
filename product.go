package main

import (
	"embed"
	"encoding/json"
)

//go:embed frontend/product.json
var productFiles embed.FS
var product = loadProduct()

func loadProduct() struct {
	Name string `json:"name"`
} {
	var p struct {
		Name string `json:"name"`
	}
	data, err := productFiles.ReadFile("frontend/product.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, &p); err != nil {
		panic(err)
	}
	if p.Name == "" {
		panic("product name is empty")
	}
	return p
}
