package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	PAGEMAP_BATCH      = 64 << 10
	PROC_KPAGECGROUP   = "/proc/kpagecgroup"
	PAGEMAP_LENGTH     = int64(8)
	SIZEOF_INT64       = 8 // bytes
	PFN_MASK           = 0x7FFFFFFFFFFFFF
	CGROUP_MOUNT_POINT = "/sys/fs/cgroup/memory/"
)

var (
	pageSize = int64(syscall.Getpagesize())
	debug    bool
)

func init() {
	flag.BoolVar(&debug, "debug", false, "full log")

}

type Cgroup struct {
	path  string
	inode uint64
}

type Hist struct {
	c uint64         // count charged mmaped
	h map[uint64]int // cgroup inode to count
	p int64          // count pages
}

func loadCgroups() (map[uint64]Cgroup, error) {
	cgroups := make(map[uint64]Cgroup)
	err := filepath.Walk(CGROUP_MOUNT_POINT, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			var stat syscall.Stat_t
			err := syscall.Stat(path, &stat)
			if err != nil {
				return err
			}
			c := Cgroup{
				path:  path,
				inode: stat.Ino,
			}
			cgroups[stat.Ino] = c
		}
		return nil
	})
	return cgroups, err
}

func fileCgroups(path string) (*Hist, error) {
	fmt.Println("Working with file: ", path)

	// open file
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NOATIME, 0400)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// get size
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	end := stat.Size()

	size := int64(0)

	// hist
	hist := Hist{
		p: end/pageSize + 1,
		h: make(map[uint64]int),
	}

	// open pagemap
	pagemap, err := os.OpenFile("/proc/self/pagemap", os.O_RDONLY, 0400)
	if err != nil {
		log.Fatalln("Open /proc/self/pagemap: ", err)
	}
	defer pagemap.Close()

	// open kpagecgroup
	kpagecgroup, err := os.OpenFile(PROC_KPAGECGROUP, os.O_RDONLY, 0400)
	if err != nil {
		log.Fatalf("Open %s: %s", PROC_KPAGECGROUP, err)
	}
	defer kpagecgroup.Close()

	for off := int64(0); off < end; off += size {
		nr := (end - off + pageSize - 1) / pageSize
		if nr > PAGEMAP_BATCH {
			nr = PAGEMAP_BATCH
		}
		size = nr * pageSize

		// mmap file
		b, err := syscall.Mmap(int(file.Fd()), off, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
		defer syscall.Munmap(b)

		// disable readahead
		err = syscall.Madvise(b, syscall.MADV_RANDOM)
		if err != nil {
			return nil, err
		}

		// mincore for finding out pages in page cache
		bPtr := uintptr(unsafe.Pointer(&b[0]))

		vecsz := (size + int64(pageSize) - 1) / int64(pageSize)
		vec := make([]byte, vecsz)

		sizePtr := uintptr(size)
		vecPtr := uintptr(unsafe.Pointer(&vec[0]))

		ret, _, err := syscall.Syscall(syscall.SYS_MINCORE, bPtr, sizePtr, vecPtr)
		if ret != 0 {
			return nil, fmt.Errorf("syscall SYS_MINCORE failed: %v", err)
		}

		// hack for get pages in our pagemap
		for i, v := range vec {
			if v%2 == 1 {
				// only if it was in page cache
				_ = *(*int)(unsafe.Pointer(bPtr + uintptr(pageSize*int64(i))))
			}
		}

		// enable readahead
		err = syscall.Madvise(b, syscall.MADV_SEQUENTIAL)
		if err != nil {
			return nil, err
		}

		// read pagemap
		index := int64(bPtr) / pageSize * PAGEMAP_LENGTH
		buf := make([]byte, nr*PAGEMAP_LENGTH)

		n, err := pagemap.ReadAt(buf, index)
		if err != nil {
			return nil, err
		}

		if int64(n)/PAGEMAP_LENGTH != nr {
			return nil, fmt.Errorf("read data from pagemap invalid")
		}

		data := make([]uint64, len(buf)/SIZEOF_INT64)
		for i := range data {
			data[i] = binary.LittleEndian.Uint64(buf[i*SIZEOF_INT64 : (i+1)*SIZEOF_INT64])
		}

		for _, d := range data {
			pfn := d & PFN_MASK
			if pfn == 0 {
				continue
			}

			cgroup := make([]byte, 8)
			n, err := kpagecgroup.ReadAt(cgroup, int64(pfn)*PAGEMAP_LENGTH)
			if err != nil {
				return nil, err
			}

			if int64(n/8) != 1 {
				return nil, fmt.Errorf("read data from /proc/kpagecgroup is invalid")
			}

			c := binary.LittleEndian.Uint64(cgroup)
			if _, ok := hist.h[c]; ok {
				hist.h[c] += 1
			} else {
				hist.h[c] = 1
			}
			hist.c += 1

			if debug {
				fmt.Printf("cgroup memory inode for pfn %x: %d\n", pfn, c)
			}
		}
	}

	return &hist, nil
}

func main() {
	flag.Parse()

	cgroups, err := loadCgroups()
	if err != nil {
		log.Fatalln(err)
	}

	hist, err := fileCgroups(flag.Arg(0))
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("total pages: %d\n", hist.p)
	fmt.Printf("total mmaped pages: %d\n", hist.c)
	fmt.Printf("total unmapped pages: %d\n", uint64(hist.p)-hist.c)

	for k, v := range hist.h {
		c := cgroups[k]
		fmt.Printf("cgroup %s (inode: %d) charges: %d pages (%d%%)\n", c.path, c.inode, v, int64(v)*int64(100)/hist.p)
	}
}
