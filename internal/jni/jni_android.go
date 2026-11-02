//go:build android

package jni

/*
#cgo CFLAGS: -Werror

#include <jni.h>
#include <stdlib.h>
#include <string.h>

static jint ayra_attach(JavaVM *vm, JNIEnv **env) {
	jint r = (*vm)->GetEnv(vm, (void **)env, JNI_VERSION_1_6);
	if (r == JNI_OK) {
		return 0;
	}
	if (r != JNI_EDETACHED) {
		return -1;
	}
	if ((*vm)->AttachCurrentThread(vm, env, NULL) != JNI_OK) {
		return -1;
	}
	return 1;
}

static void ayra_detach(JavaVM *vm) { (*vm)->DetachCurrentThread(vm); }

static jclass ayra_getObjectClass(JNIEnv *env, jobject o) { return (*env)->GetObjectClass(env, o); }
static jmethodID ayra_getMethodID(JNIEnv *env, jclass c, const char *n, const char *s) { return (*env)->GetMethodID(env, c, n, s); }
static jmethodID ayra_getStaticMethodID(JNIEnv *env, jclass c, const char *n, const char *s) { return (*env)->GetStaticMethodID(env, c, n, s); }
static jobject ayra_callObjectMethodA(JNIEnv *env, jobject o, jmethodID m, const jvalue *a) { return (*env)->CallObjectMethodA(env, o, m, a); }
static void ayra_callStaticVoidMethodA(JNIEnv *env, jclass c, jmethodID m, const jvalue *a) { (*env)->CallStaticVoidMethodA(env, c, m, a); }
static jobject ayra_callStaticObjectMethodA(JNIEnv *env, jclass c, jmethodID m, const jvalue *a) { return (*env)->CallStaticObjectMethodA(env, c, m, a); }
static void ayra_callVoidMethodA(JNIEnv *env, jobject o, jmethodID m, const jvalue *a) { (*env)->CallVoidMethodA(env, o, m, a); }
static jint ayra_callIntMethodA(JNIEnv *env, jobject o, jmethodID m, const jvalue *a) { return (*env)->CallIntMethodA(env, o, m, a); }
static jboolean ayra_callBooleanMethodA(JNIEnv *env, jobject o, jmethodID m, const jvalue *a) { return (*env)->CallBooleanMethodA(env, o, m, a); }
static jobject ayra_callObjectMethodA2(JNIEnv *env, jobject o, jmethodID m, const jvalue *a) { return (*env)->CallObjectMethodA(env, o, m, a); }
static jint ayra_callStaticIntMethodA(JNIEnv *env, jclass c, jmethodID m, const jvalue *a) { return (*env)->CallStaticIntMethodA(env, c, m, a); }
static jboolean ayra_callStaticBooleanMethodA(JNIEnv *env, jclass c, jmethodID m, const jvalue *a) { return (*env)->CallStaticBooleanMethodA(env, c, m, a); }
static jlong ayra_callStaticLongMethodA(JNIEnv *env, jclass c, jmethodID m, const jvalue *a) { return (*env)->CallStaticLongMethodA(env, c, m, a); }
static jobject ayra_newGlobalRef(JNIEnv *env, jobject o) { return (*env)->NewGlobalRef(env, o); }
static void ayra_deleteGlobalRef(JNIEnv *env, jobject o) { (*env)->DeleteGlobalRef(env, o); }
static void ayra_deleteLocalRef(JNIEnv *env, jobject o) { (*env)->DeleteLocalRef(env, o); }
static jstring ayra_newStringUTF(JNIEnv *env, const char *s) { return (*env)->NewStringUTF(env, s); }
static const char *ayra_getStringUTFChars(JNIEnv *env, jstring s) { return (*env)->GetStringUTFChars(env, s, NULL); }
static void ayra_releaseStringUTFChars(JNIEnv *env, jstring s, const char *c) { (*env)->ReleaseStringUTFChars(env, s, c); }
static jsize ayra_getArrayLength(JNIEnv *env, jarray a) { return (*env)->GetArrayLength(env, a); }
static jobject ayra_getObjectArrayElement(JNIEnv *env, jobjectArray a, jsize i) { return (*env)->GetObjectArrayElement(env, a, i); }
static jobjectArray ayra_newObjectArray(JNIEnv *env, jsize n, jclass c, jobject init) { return (*env)->NewObjectArray(env, n, c, init); }
static void ayra_setObjectArrayElement(JNIEnv *env, jobjectArray a, jsize i, jobject v) { (*env)->SetObjectArrayElement(env, a, i, v); }
static jbyteArray ayra_newByteArray(JNIEnv *env, jsize n) { return (*env)->NewByteArray(env, n); }
static void ayra_setByteArrayRegion(JNIEnv *env, jbyteArray a, jsize s, jsize n, const void *b) { (*env)->SetByteArrayRegion(env, a, s, n, (const jbyte *)b); }
static void ayra_getByteArrayRegion(JNIEnv *env, jbyteArray a, jsize s, jsize n, void *b) { (*env)->GetByteArrayRegion(env, a, s, n, (jbyte *)b); }
static void *ayra_getDirectBufferAddress(JNIEnv *env, jobject b) { return (*env)->GetDirectBufferAddress(env, b); }
static jlong ayra_getDirectBufferCapacity(JNIEnv *env, jobject b) { return (*env)->GetDirectBufferCapacity(env, b); }
static jboolean ayra_exceptionCheck(JNIEnv *env) { return (*env)->ExceptionCheck(env); }
static void ayra_exceptionDescribe(JNIEnv *env) { (*env)->ExceptionDescribe(env); }
static void ayra_exceptionClear(JNIEnv *env) { (*env)->ExceptionClear(env); }
static jthrowable ayra_exceptionOccurred(JNIEnv *env) { return (*env)->ExceptionOccurred(env); }

static jvalue ayra_valueObject(jobject o) { jvalue v; memset(&v, 0, sizeof v); v.l = o; return v; }
static jvalue ayra_valueInt(jint i) { jvalue v; memset(&v, 0, sizeof v); v.i = i; return v; }
static jvalue ayra_valueLong(jlong l) { jvalue v; memset(&v, 0, sizeof v); v.j = l; return v; }
static jvalue ayra_valueBool(jboolean b) { jvalue v; memset(&v, 0, sizeof v); v.z = b; return v; }
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/arandu-io/ayra/engine/app"
)

// Env is the JNIEnv of the thread a [Do] is running on. It is valid only inside
// that call.
type Env struct{ p *C.JNIEnv }

// Ref is a global reference to a Java object, valid on any thread until
// released.
type Ref uintptr

// Class is a global reference to a Java class.
type Class Ref

// Value is one argument of a call.
type Value struct{ v C.jvalue }

// Obj is an object argument, a reference of any kind.
func Obj(r uintptr) Value { return Value{C.ayra_valueObject(C.jobject(r))} }

// Int is an int argument.
func Int(i int) Value { return Value{C.ayra_valueInt(C.jint(i))} }

// Long is a long argument.
func Long(l int64) Value { return Value{C.ayra_valueLong(C.jlong(l))} }

// Bool is a boolean argument.
func Bool(b bool) Value {
	if b {
		return Value{C.ayra_valueBool(C.JNI_TRUE)}
	}
	return Value{C.ayra_valueBool(C.JNI_FALSE)}
}

// ErrNotRunning is a call made before the Java half has started the Go
// runtime: there is no virtual machine to call into.
var ErrNotRunning = errors.New("jni: the Java virtual machine is not available yet")

// Do runs f with the JNIEnv of the calling thread, attaching the thread to the
// virtual machine for the duration when it is not attached already.
//
// f runs on one thread for its whole length: Do holds the goroutine to it.
//
// Local references made inside f are released when it returns, by the machine
// for a thread this attached and by the caller of a native method for one that
// was already attached.
func Do(f func(Env) error) error {
	// A JNIEnv belongs to one operating-system thread, and a goroutine is moved
	// between threads whenever the scheduler likes. Without the lock the calls
	// of one Do could be made from three threads with the environment of the
	// first: the machine answers with StackOverflowError for a call made from a
	// thread whose stack it never recorded, and then aborts on a local reference
	// it finds in no table.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// The machine's address arrives as an integer and is read back as the
	// pointer it was, without a conversion the vet check cannot tell from a
	// forged one.
	address := app.JavaVM()
	vm := *(**C.JavaVM)(unsafe.Pointer(&address))
	if vm == nil {
		return ErrNotRunning
	}
	var env *C.JNIEnv
	switch C.ayra_attach(vm, &env) {
	case 0:
	case 1:
		defer C.ayra_detach(vm)
	default:
		return errors.New("jni: the thread could not be attached to the virtual machine")
	}
	return f(Env{env})
}

// EnvOf wraps the JNIEnv a native method was called with, for the exported
// functions that receive it as an argument.
func EnvOf(p uintptr) Env { return Env{*(**C.JNIEnv)(unsafe.Pointer(&p))} }

// Context is the application context as a reference valid in this call.
func Context() uintptr { return app.AppContext() }

// classes remembers the classes found, because loading one goes through the
// application's class loader and that is not free.
var (
	classMu sync.Mutex
	classes = map[string]Class{}
)

// Class finds a class of the application by its dotted name.
//
// The thread a Go goroutine runs on was attached by this package and not by the
// virtual machine, so its class loader is the system one, which knows nothing
// of the application's classes. The application's own loader is asked instead,
// through the context.
func (e Env) Class(name string) (Class, error) {
	classMu.Lock()
	defer classMu.Unlock()
	if c, ok := classes[name]; ok {
		return c, nil
	}
	ctx := C.jobject(Context())
	if ctx == 0 {
		return 0, ErrNotRunning
	}
	ctxClass := C.ayra_getObjectClass(e.p, ctx)
	defer C.ayra_deleteLocalRef(e.p, C.jobject(ctxClass))

	getLoader, err := e.methodID(Class(ctxClassRef(e, ctxClass)), "getClassLoader", "()Ljava/lang/ClassLoader;", false)
	if err != nil {
		return 0, err
	}
	loader := C.ayra_callObjectMethodA(e.p, ctx, getLoader, nil)
	if err := e.check(); err != nil {
		return 0, err
	}
	defer C.ayra_deleteLocalRef(e.p, loader)
	loaderClass := C.ayra_getObjectClass(e.p, loader)
	defer C.ayra_deleteLocalRef(e.p, C.jobject(loaderClass))
	loadClass, err := e.methodID(Class(ctxClassRef(e, loaderClass)), "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;", false)
	if err != nil {
		return 0, err
	}
	jname := e.String(name)
	defer C.ayra_deleteLocalRef(e.p, C.jobject(jname))
	arg := Obj(jname)
	found := C.ayra_callObjectMethodA(e.p, loader, loadClass, &arg.v)
	if err := e.check(); err != nil {
		return 0, fmt.Errorf("jni: class %s: %w", name, err)
	}
	if found == 0 {
		return 0, fmt.Errorf("jni: class %s was not found", name)
	}
	global := C.ayra_newGlobalRef(e.p, found)
	C.ayra_deleteLocalRef(e.p, found)
	c := Class(uintptr(global))
	classes[name] = c
	return c, nil
}

// ctxClassRef lets a local class reference stand where a Class is expected for
// the one call that finds a method on it.
func ctxClassRef(_ Env, c C.jclass) uintptr { return uintptr(c) }

func (e Env) methodID(c Class, name, sig string, static bool) (C.jmethodID, error) {
	n, s := C.CString(name), C.CString(sig)
	defer C.free(unsafe.Pointer(n))
	defer C.free(unsafe.Pointer(s))
	cls := C.jclass(c)
	var id C.jmethodID
	if static {
		id = C.ayra_getStaticMethodID(e.p, cls, n, s)
	} else {
		id = C.ayra_getMethodID(e.p, cls, n, s)
	}
	if err := e.check(); err != nil {
		return nil, fmt.Errorf("jni: method %s%s: %w", name, sig, err)
	}
	if id == nil {
		return nil, fmt.Errorf("jni: method %s%s was not found", name, sig)
	}
	return id, nil
}

// check turns a pending Java exception into an error, and clears it. The
// exception is described to the log first, which is where somebody looking for
// the cause goes.
func (e Env) check() error {
	if C.ayra_exceptionCheck(e.p) == C.JNI_FALSE {
		return nil
	}
	C.ayra_exceptionDescribe(e.p)
	C.ayra_exceptionClear(e.p)
	return errors.New("a Java exception was thrown; see the log")
}

func args(a []Value) *C.jvalue {
	if len(a) == 0 {
		return nil
	}
	return &a[0].v
}

// CallStaticVoid calls a static method that returns nothing.
func (e Env) CallStaticVoid(c Class, name, sig string, a ...Value) error {
	id, err := e.methodID(c, name, sig, true)
	if err != nil {
		return err
	}
	C.ayra_callStaticVoidMethodA(e.p, C.jclass(c), id, args(a))
	return e.check()
}

// CallStaticInt calls a static method that returns an int.
func (e Env) CallStaticInt(c Class, name, sig string, a ...Value) (int, error) {
	id, err := e.methodID(c, name, sig, true)
	if err != nil {
		return 0, err
	}
	r := C.ayra_callStaticIntMethodA(e.p, C.jclass(c), id, args(a))
	return int(r), e.check()
}

// CallStaticBool calls a static method that returns a boolean.
func (e Env) CallStaticBool(c Class, name, sig string, a ...Value) (bool, error) {
	id, err := e.methodID(c, name, sig, true)
	if err != nil {
		return false, err
	}
	r := C.ayra_callStaticBooleanMethodA(e.p, C.jclass(c), id, args(a))
	return r == C.JNI_TRUE, e.check()
}

// CallStaticLong calls a static method that returns a long.
func (e Env) CallStaticLong(c Class, name, sig string, a ...Value) (int64, error) {
	id, err := e.methodID(c, name, sig, true)
	if err != nil {
		return 0, err
	}
	r := C.ayra_callStaticLongMethodA(e.p, C.jclass(c), id, args(a))
	return int64(r), e.check()
}

// CallStaticObject calls a static method that returns an object, answered as a
// local reference that is valid until the enclosing [Do] returns.
func (e Env) CallStaticObject(c Class, name, sig string, a ...Value) (uintptr, error) {
	id, err := e.methodID(c, name, sig, true)
	if err != nil {
		return 0, err
	}
	r := C.ayra_callStaticObjectMethodA(e.p, C.jclass(c), id, args(a))
	return uintptr(r), e.check()
}

// CallVoid calls a method of an object that returns nothing.
func (e Env) CallVoid(obj Ref, c Class, name, sig string, a ...Value) error {
	id, err := e.methodID(c, name, sig, false)
	if err != nil {
		return err
	}
	C.ayra_callVoidMethodA(e.p, C.jobject(obj), id, args(a))
	return e.check()
}

// CallInt calls a method of an object that returns an int.
func (e Env) CallInt(obj Ref, c Class, name, sig string, a ...Value) (int, error) {
	id, err := e.methodID(c, name, sig, false)
	if err != nil {
		return 0, err
	}
	r := C.ayra_callIntMethodA(e.p, C.jobject(obj), id, args(a))
	return int(r), e.check()
}

// CallBool calls a method of an object that returns a boolean.
func (e Env) CallBool(obj Ref, c Class, name, sig string, a ...Value) (bool, error) {
	id, err := e.methodID(c, name, sig, false)
	if err != nil {
		return false, err
	}
	r := C.ayra_callBooleanMethodA(e.p, C.jobject(obj), id, args(a))
	return r == C.JNI_TRUE, e.check()
}

// CallObject calls a method of an object that returns an object, answered as a
// local reference.
func (e Env) CallObject(obj Ref, c Class, name, sig string, a ...Value) (uintptr, error) {
	id, err := e.methodID(c, name, sig, false)
	if err != nil {
		return 0, err
	}
	r := C.ayra_callObjectMethodA2(e.p, C.jobject(obj), id, args(a))
	return uintptr(r), e.check()
}

// String makes a Java string, as a local reference.
func (e Env) String(s string) uintptr {
	c := C.CString(s)
	defer C.free(unsafe.Pointer(c))
	return uintptr(C.ayra_newStringUTF(e.p, c))
}

// GoString reads a Java string. A null reference is the empty string.
func (e Env) GoString(ref uintptr) string {
	if ref == 0 {
		return ""
	}
	s := C.jstring(ref)
	chars := C.ayra_getStringUTFChars(e.p, s)
	if chars == nil {
		return ""
	}
	defer C.ayra_releaseStringUTFChars(e.p, s, chars)
	return C.GoString(chars)
}

// Strings reads a Java String[] into a slice. A null array is empty.
func (e Env) Strings(array uintptr) []string {
	if array == 0 {
		return nil
	}
	arr := C.jobjectArray(array)
	n := int(C.ayra_getArrayLength(e.p, C.jarray(arr)))
	out := make([]string, n)
	for i := range out {
		el := C.ayra_getObjectArrayElement(e.p, arr, C.jsize(i))
		out[i] = e.GoString(uintptr(el))
		C.ayra_deleteLocalRef(e.p, el)
	}
	return out
}

// Bytes makes a Java byte[] from a slice, as a local reference.
func (e Env) Bytes(b []byte) uintptr {
	arr := C.ayra_newByteArray(e.p, C.jsize(len(b)))
	if arr != 0 && len(b) > 0 {
		C.ayra_setByteArrayRegion(e.p, arr, 0, C.jsize(len(b)), unsafe.Pointer(&b[0]))
	}
	return uintptr(arr)
}

// ByteArrays makes a Java byte[][] from slices, as a local reference.
func (e Env) ByteArrays(parts [][]byte) (uintptr, error) {
	cls := e.findSystemClass("[B")
	if cls == 0 {
		return 0, errors.New("jni: byte[] was not found")
	}
	defer C.ayra_deleteLocalRef(e.p, C.jobject(cls))
	outer := C.ayra_newObjectArray(e.p, C.jsize(len(parts)), cls, 0)
	for i, part := range parts {
		inner := e.Bytes(part)
		C.ayra_setObjectArrayElement(e.p, outer, C.jsize(i), C.jobject(inner))
		C.ayra_deleteLocalRef(e.p, C.jobject(inner))
	}
	return uintptr(outer), e.check()
}

// Buffer is the memory behind a direct ByteBuffer: a slice that aliases it. It
// is valid only while the buffer is, which is the caller's to know.
func (e Env) Buffer(buffer uintptr) []byte {
	if buffer == 0 {
		return nil
	}
	p := C.ayra_getDirectBufferAddress(e.p, C.jobject(buffer))
	n := int(C.ayra_getDirectBufferCapacity(e.p, C.jobject(buffer)))
	if p == nil || n <= 0 {
		return nil
	}
	return unsafe.Slice((*byte)(p), n)
}

// Global makes a global reference of a local one.
func (e Env) Global(local uintptr) Ref {
	return Ref(uintptr(C.ayra_newGlobalRef(e.p, C.jobject(local))))
}

// Release drops a global reference.
func (e Env) Release(r Ref) {
	if r != 0 {
		C.ayra_deleteGlobalRef(e.p, C.jobject(uintptr(r)))
	}
}

// DeleteLocal drops a local reference made inside a long-running [Do].
func (e Env) DeleteLocal(ref uintptr) {
	if ref != 0 {
		C.ayra_deleteLocalRef(e.p, C.jobject(ref))
	}
}

func (e Env) findSystemClass(name string) C.jclass {
	return findClass(e.p, name)
}
