package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"gvisor.dev/gvisor/pkg/erofs"
	"gvisor.dev/gvisor/pkg/log"
	"gvisor.dev/gvisor/pkg/safemem"
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

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	log.Log().SetLevel(log.Debug)

	img, err := erofs.OpenImage(f)
	if err != nil {
		return fmt.Errorf("opening EROFS image: %w", err)
	}
	defer img.Close()

	if err := printSuperBlock(img.SuperBlock()); err != nil {
		return fmt.Errorf("printing super block: %w", err)
	}

	root, err := img.Inode(img.RootNid())
	if err != nil {
		return fmt.Errorf("getting root inode: %w", err)
	}

	if err := printInode(root); err != nil {
		return fmt.Errorf("printing root inode: %w", err)
	}

	if err := inodePathsRec(img, root, "/", nil); err != nil {
		return fmt.Errorf("iterating dirents: %w", err)
	}

	return nil
}

func printSuperBlock(sb erofs.SuperBlock) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	defer w.Flush()

	var err error
	fsUUID, err := uuid.FromBytes(sb.UUID[:])
	if err != nil {
		errors.Join(err, fmt.Errorf("parsing uuid: %w", err))
	}

	fmt.Fprintf(w, "magic number:\t0x%X\n", sb.Magic)
	fmt.Fprintf(w, "checksum:\t%x\n", sb.Checksum)
	fmt.Fprintf(w, "feature compat:\t%d\n", sb.FeatureCompat)
	fmt.Fprintf(w, "block size:\t%d bits\n", int(math.Pow(2, float64(sb.BlockSizeBits))))
	fmt.Fprintf(w, "extension slots:\t%d\n", sb.ExtSlots)
	fmt.Fprintf(w, "root nid:\t%d\n", sb.RootNid)
	fmt.Fprintf(w, "inodes:\t%d\n", sb.Inodes)
	fmt.Fprintf(w, "build time:\t%s\n", time.Unix(int64(sb.BuildTime), int64(sb.BuildTimeNsec)).Format(time.ANSIC))
	fmt.Fprintf(w, "build time sec, nsec:\t%d, %d\n", sb.BuildTime, sb.BuildTimeNsec)
	fmt.Fprintf(w, "blocks:\t%d\n", sb.Blocks)
	fmt.Fprintf(w, "meta block address:\t%d\n", sb.MetaBlockAddr)
	fmt.Fprintf(w, "xattr block address:\t%d\n", sb.XattrBlockAddr)
	fmt.Fprintf(w, "uuid:\t%s\n", fsUUID.String())
	fmt.Fprintf(w, "volume name:\t%s\n", sb.VolumeName)
	fmt.Fprintf(w, "feature incompat:\t%d\n", sb.FeatureIncompat)
	fmt.Fprintf(w, "union1:\t%d\n", sb.Union1)
	fmt.Fprintf(w, "extra devices:\t%d\n", sb.ExtraDevices)
	fmt.Fprintf(w, "device table slot offset:\t%d\n", sb.DevTableSlotOff)
	if !bytes.Equal(sb.Reserved[:], make([]byte, 38)) {
		fmt.Fprintf(w, "reserved:\t%x\n", sb.Reserved)
	}

	return err
}

func printInode(i erofs.Inode) error {
	// Path : /tmp
	// Size: 27  On-disk size: 27  directory
	// NID: 100   Links: 2   Layout: 2   Compression ratio: 100.00%
	// Inode size: 64   Xattr size: 0
	// Uid: 0   Gid: 0  Access: 0755/rwxr-xr-x
	// Timestamp: 1980-01-01 01:00:00.000000000

	// Ext:   logical offset   |  length :     physical offset    |  length
	// 0:        0..      27   |      27 :       3264..      3291 |      27
	// /tmp: 1 extents found

	fmt.Printf("Size:\t%d\tOn-disk size:\t%d\t%s\n", i.Size(), i.Size(), inodeTypeStr(i))
	fmt.Printf("NID:\t%d\tLinks:\t%d\tLayout:\t%d\n", i.Nid(), i.Nlink(), i.Layout())
	fmt.Printf("Inode size:\t%d\tXattr size:\t%d\n", i.Size())
	fmt.Printf("Uid:\t%d\tGid:\t%d\tAccess:\t%#o/%s\n", i.UID(), i.GID(), i.Mode(), permToString(i.Mode()))
	fmt.Printf("Timestamp:\t%s\n", time.Unix(int64(i.Mtime()), int64(i.MtimeNsec())).Format("2006-01-02 15:04:05.000000000"))
	fmt.Println("Ext:\tlogical offset\t|\tlength\t:\tphysical offset\t|\tlength")
	data, err := i.Data()
	if err != nil {
		return fmt.Errorf("getting data: %w", err)
	}
	hash := sha256.New()
	io.Copy(hash, safemem.ToIOReader{Reader: &safemem.BlockSeqReader{Blocks: data}})
	fmt.Printf("file content sha256: %x\n", hash.Sum(nil))
	// fmt.Printf("%d:\t%d..%d\t|\t%d\t:\t%d..%d\t|\t%d\n", 0, 0, 0, 0, offset, offset+i.Size(), i.Size())
	// fmt.Printf(, "%s: %d extents found\n", i.Path(), len(i.Extents()))
	fmt.Println()
	return nil
}

func inodePathsRec(img *erofs.Image, i erofs.Inode, prefix string, seen map[uint64]struct{}) error {
	if seen == nil {
		seen = make(map[uint64]struct{})
	}
	return i.IterDirents(func(name string, typ uint8, nid uint64) error {
		if _, ok := seen[nid]; ok {
			return nil
		}
		seen[nid] = struct{}{}
		child, err := img.Inode(nid)
		fmt.Printf("Path : %s\n", filepath.Join(prefix, name))
		if err := printInode(child); err != nil {
			return fmt.Errorf("printing inode: %w", err)
		}
		if err != nil {
			return fmt.Errorf("getting inode: %w", err)
		}
		if !child.IsDir() {
			return nil
		}
		return inodePathsRec(img, child, filepath.Join(prefix, name), seen)
	})
}

func permToString(perm uint16) string {
	getRWX := func(bits uint16) string {
		var result string
		if bits&4 != 0 {
			result += "r"
		} else {
			result += "-"
		}
		if bits&2 != 0 {
			result += "w"
		} else {
			result += "-"
		}
		if bits&1 != 0 {
			result += "x"
		} else {
			result += "-"
		}
		return result
	}

	var str string
	str += getRWX((perm >> 6) & 7)
	str += getRWX((perm >> 3) & 7)
	str += getRWX(perm & 7)
	return str
}

func inodeTypeStr(i erofs.Inode) string {
	switch {
	case i.IsRegular():
		return "regular file"
	case i.IsDir():
		return "directory"
	case i.IsSymlink():
		return "symlink"
	case i.IsSocket():
		return "socket"
	case i.IsFIFO():
		return "fifo"
	case i.IsCharDev():
		return "character device"
	case i.IsBlockDev():
		return "block device"
	default:
		return "unknown"
	}
}
