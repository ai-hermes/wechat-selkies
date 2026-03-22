package main

/*
#cgo CFLAGS: -DCHATLOG_CGO
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
int find_all_keys_macos(int pid, const char *out_path);
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func FindAllKeysMacOS(pid int, outputPath string) error {
	var cPath *C.char
	if outputPath != "" {
		cPath = C.CString(outputPath)
		defer C.free(unsafe.Pointer(cPath))
	}
	ret := C.find_all_keys_macos(C.int(pid), cPath)
	if ret != 0 {
		return fmt.Errorf("find_all_keys_macos failed: %d", int(ret))
	}
	return nil
}

func main() {
	fmt.Println("xx")
	err := FindAllKeysMacOS(14351, "./xx.json")
	if err != nil {
		panic(err)
	}
}
