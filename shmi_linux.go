//go:build linux && cgo
// +build linux,cgo

package shm

/*
#cgo LDFLAGS: -lrt

#include <sys/mman.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

int _create(const char* name, int size, int flag) {
	mode_t mode = S_IRUSR | S_IWUSR  ;

	int fd = shm_open(name, flag, mode);
	if (fd < 0) {
		return -1;
	}

	if (ftruncate(fd, size) != 0) {
		close(fd);
		return -2;
	}
	return fd;
}

int Create(const char* name, int size) {
	int flag = O_RDWR | O_CREAT | O_EXCL;
	return _create(name, size, flag);
}

int Open(const char* name, int size) {
	int flag = O_RDWR;
	return _create(name, size, flag);
}

void* Map(int fd, int size) {
	void* p = mmap(
		NULL, size,
		PROT_READ | PROT_WRITE,
		MAP_SHARED, fd, 0);
	if (p == MAP_FAILED) {
		return NULL;
	}
	return p;
}

void Close(int fd, void* p, int size) {
	if (p != NULL) {
		munmap(p, size);
	}
	if (fd != 0) {
		close(fd);
	}
}

void Delete(const char* name) {
	shm_unlink(name);
}
*/
import "C"

import (
	"fmt"
	"io"
	"unsafe"
)

type shmi struct {
	cname *C.char
	fd    C.int
	v     unsafe.Pointer
	size  int32

	// true if this mod created the shm
	parent bool
}

// create shared memory. return shmi object.
func create(name string, size int32) (*shmi, error) {
	cname := C.CString(name)

	fd := C.Create(cname, C.int(size))
	if fd < 0 {
		return nil, fmt.Errorf("can't create file %s", name)
	}

	v := C.Map(fd, C.int(size))
	if v == nil {
		C.Close(fd, nil, C.int(size))
		C.Delete(cname)
		C.free(unsafe.Pointer(cname))
		return nil, fmt.Errorf("can't map file")
	}

	return &shmi{cname, fd, v, size, true}, nil
}

// open shared memory. return shmi object.
func open(name string, size int32) (*shmi, error) {
	cname := C.CString(name)

	fd := C.Open(cname, C.int(size))
	if fd < 0 {
		return nil, fmt.Errorf("open")
	}

	v := C.Map(fd, C.int(size))
	if v == nil {
		C.Close(fd, nil, C.int(size))
		C.free(unsafe.Pointer(cname))
		return nil, fmt.Errorf("can't map file")
	}

	return &shmi{cname, fd, v, size, false}, nil
}

func (o *shmi) close() error {
	if o.v != nil {
		C.Close(o.fd, o.v, C.int(o.size))
		o.v = nil
	}
	if o.parent {
		C.Delete(o.cname)
	}
	C.free(unsafe.Pointer(o.cname))

	return nil
}

// read shared memory. return read size.
func (o *shmi) readAt(p []byte, off int64) (n int, err error) {
	if off >= int64(o.size) {
		return 0, io.EOF
	}
	if max := int64(o.size) - off; int64(len(p)) > max {
		p = p[:max]
	}
	return copyPtr2Slice(uintptr(o.v), p, off, o.size), nil
}

// write shared memory. return write size.
func (o *shmi) writeAt(p []byte, off int64) (n int, err error) {
	if off >= int64(o.size) {
		return 0, io.EOF
	}
	if max := int64(o.size) - off; int64(len(p)) > max {
		p = p[:max]
	}
	return copySlice2Ptr(p, uintptr(o.v), off, o.size), nil
}
