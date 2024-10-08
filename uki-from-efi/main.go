package main

import (
	"fmt"
	"io"
	"os"

	"github.com/diskfs/go-diskfs/filesystem/fat32"
)

const ukiPath = "/EFI/BOOT/BOOTX64.EFI"

func main() {
	if err := run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	path := os.Args[1]

	efiPart, err := os.Open(path)
	if err != nil {
		return err
	}
	defer efiPart.Close()

	efiPartInfo, err := efiPart.Stat()
	if err != nil {
		return err
	}

	fatFS, err := fat32.Read(efiPart, efiPartInfo.Size(), 0, 512)
	if err != nil {
		return err
	}

	ukiFile, err := fatFS.OpenFile(ukiPath, os.O_RDONLY)
	if err != nil {
		return err
	}
	defer ukiFile.Close()

	_, ukiFileSize, err := (ukiFile.(*fat32.File)).GetContentSection()
	if err != nil {
		return err
	}

	ukiTarget, err := os.OpenFile("uki", os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer ukiTarget.Close()

	n, err := io.Copy(ukiTarget, ukiFile)
	if err != nil {
		return err
	} else if n != ukiFileSize {
		return fmt.Errorf("copied %d bytes, expected %d", n, ukiFileSize)
	}

	return nil
}
