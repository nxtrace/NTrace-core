package config

import (
	"testing"
	"log"
)

func TestReadConfig(t *testing.T) {
	if res, err := Read(); err != nil {
		log.Println(err)
	} else {
		log.Println(res)
	}
}

func TestGenerateConfig(t *testing.T) {
	Generate()
}