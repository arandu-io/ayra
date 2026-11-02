// Package jni is the thin layer the native bridges of this module call the Java
// half of an Android application through.
//
// It is for the packages that carry Java of their own -- push notifications and
// the video decoder -- and not for an application. What it offers is exactly
// what those need: attach the calling thread, find a class of the application,
// call a static method, make and read strings, byte arrays and direct buffers.
// The window's own bridge, in the engine, has its own copy of the JNI calls and
// keeps them, because it runs before this module's packages do and shares no
// state with them.
//
// Every reference this package returns is a global reference, so it is valid on
// any thread until [Release] is called; a local reference never leaves a [Do].
package jni
