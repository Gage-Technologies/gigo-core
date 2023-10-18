package utils

import (
	"math/rand"
	"testing"
)

func TestGenerateInternalServicePassword(t *testing.T) {
	rand.Seed(42069)

	// p, err := GenerateInternalServicePassword(rand.Int63())
	// if err != nil {
	// 	t.Errorf("\nGenerateInternalServicePassword failed\n    Error: %v", err)
	// 	return
	// }

	p, err := GenerateInternalServicePassword(1611445184652902400)
	if err != nil {
		t.Errorf("\nGenerateInternalServicePassword failed\n    Error: %v", err)
		return
	}

	if p != "f3fa759870053bf69b0c5a11bf6232d1d3bdbc284d26c9a3bfba5e0bdd4c67bc" {
		t.Errorf("\nGenerateInternalServicePassword failed\n    Error: incorrect password %q", p)
		return
	}

	p, err = GenerateInternalServicePassword(rand.Int63())
	if err != nil {
		t.Errorf("\nGenerateInternalServicePassword failed\n    Error: %v", err)
		return
	}

	if p != "af3dfbee36fb0ffbf679e82b7edaa710a3a7f7e7a5eca4bb85faac8a44aa9b3a" {
		t.Errorf("\nGenerateInternalServicePassword failed\n    Error: incorrect password %q", p)
		return
	}

	p, err = GenerateInternalServicePassword(rand.Int63())
	if err != nil {
		t.Errorf("\nGenerateInternalServicePassword failed\n    Error: %v", err)
		return
	}

	if p != "a26c7b5d2ac05690f9c53e0b7134054de4e73fb3ca3e8175701f296edbff2f97" {
		t.Errorf("\nGenerateInternalServicePassword failed\n    Error: incorrect password %q", p)
		return
	}

	t.Log("\nGenerateInternalServicePassword succeeded")
}
