//go:build android

package jni

/*
#include <jni.h>
#include <stdlib.h>

static jclass ayra_findClass(JNIEnv *env, const char *name) { return (*env)->FindClass(env, name); }
*/
import "C"

import "unsafe"

// findClass finds a class of the platform's own, which the system loader knows.
func findClass(env *C.JNIEnv, name string) C.jclass {
	n := C.CString(name)
	defer C.free(unsafe.Pointer(n))
	return C.ayra_findClass(env, n)
}
