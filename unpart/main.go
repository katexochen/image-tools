package main

import (
	"fmt"
	"os"

	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/partition/gpt"
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

	disk, err := diskfs.Open(path, diskfs.WithOpenMode(diskfs.ReadOnly))
	if err != nil {
		return err
	}
	defer disk.File.Close()
	table, err := disk.GetPartitionTable()
	if err != nil {
		return err
	}
	gptTable, ok := table.(*gpt.Table)
	if !ok {
		return fmt.Errorf("partition table is not GPT")
	}
	if err := inspect(gptTable); err != nil {
		return err
	}
	if err := explode(disk.File, gptTable); err != nil {
		return err
	}
	return nil
}

func inspect(gptTable *gpt.Table) error {
	for _, partition := range gptTable.Partitions {
		fmt.Printf("Partition %s:\n", partition.Name)
		fmt.Printf("  type: %s\n", partition.Type)
		fmt.Printf("  size: %d bytes\n", partition.Size)
		fmt.Printf("  start: %d\n", partition.Start)
		fmt.Printf("  end: %d\n", partition.End)
		fmt.Printf("  guid: %s\n", partition.GUID)
	}
	return nil
}

func explode(diskFile *os.File, gptTable *gpt.Table) error {
	for _, partition := range gptTable.Partitions {
		fmt.Printf("Extracting partition %s\n", partition.Name)

		f, err := os.OpenFile(fmt.Sprintf("%s.part", partition.Name), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
		if err != nil {
			return fmt.Errorf("opening new file for partition %s: %w", partition.Name, err)
		}
		defer f.Close()
		if n, err := partition.ReadContents(diskFile, f); err != nil {
			return fmt.Errorf("copying partition %s: %w", partition.Name, err)
		} else if n != int64(partition.Size) {
			return fmt.Errorf("writing partition %s: wrote %d bytes, expected %d", partition.Name, n, partition.Size)
		}
	}
	return nil
}
