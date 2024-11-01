package main

import (
	"crypto/sha256"
	"debug/pe"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: %s <path>", os.Args[0])
	}
	path := os.Args[1]

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	pef, err := pe.NewFile(f)
	if err != nil {
		return err
	}
	defer pef.Close()
	if err := inspect(pef.Sections); err != nil {
		return err
	}

	if err := explode(pef.Sections); err != nil {
		return err
	}

	return nil
}

func openPE(path string) (*pe.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return pe.NewFile(f)
}

func inspect(sections []*pe.Section) error {
	for _, section := range sections {
		sectionType := sectionTypeOf(section.Name)
		if sectionType == "unknown" {
			continue
		}
		fmt.Printf("%s:\n", section.Name)
		fmt.Printf("  size: %d bytes\n", section.VirtualSize)
		r := section.Open()
		data := make([]byte, section.VirtualSize)
		if n, err := r.Read(data); err != nil {
			return fmt.Errorf("getting data of section %s: %w", section.Name, err)
		} else if n != len(data) {
			return fmt.Errorf("getting data of section %s: read %d bytes, expected %d", section.Name, n, len(data))
		}
		digest := sha256.Sum256(data)
		fmt.Printf("  sha256: %x\n", digest)
		if sectionTypeOf(section.Name) == "text" {
			text := string(data)
			text = strings.TrimRight(text, "\n")
			text = strings.ReplaceAll(text, "\x00", "")
			text = strings.ReplaceAll(text, "\n", "\n    ")
			fmt.Println("  text:")
			fmt.Printf("    %s\n", text)
		}
	}
	return nil
}

func explode(sections []*pe.Section) error {
	for _, section := range sections {
		sectionType := sectionTypeOf(section.Name)
		if sectionType == "unknown" {
			continue
		}
		outPath := strings.TrimPrefix(section.Name, ".")
		r := section.Open()
		f, err := os.OpenFile(outPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("opening %s: %w", outPath, err)
		}
		defer f.Close()
		if n, err := io.CopyN(f, r, int64(section.VirtualSize)); err != nil {
			return fmt.Errorf("writing %s: %w", outPath, err)
		} else if n != int64(section.VirtualSize) {
			return fmt.Errorf("writing %s: wrote %d bytes, expected %d", outPath, n, section.VirtualSize)
		}
	}
	return nil
}

func sectionTypeOf(sectionName string) string {
	switch sectionName {
	case ".cmdline", ".osrel", ".uname", ".pcrpkey", ".pcrsig", ".sbat":
		return "text"
	case ".linux", ".initrd", ".ucode", ".splash", ".dtb", ".sbom":
		return "binary"
	default:
		return "unknown"
	}
}
