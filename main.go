package main

import (
	"log"
	"goods.ru/grab-it/grabers/compyou"
)

func main() {
	if err := compyou.Run(); err != nil {
		log.Fatal(err);
	}
}
