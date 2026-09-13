//go:build ios

package compiled

/*
#include <TargetConditionals.h>

static int ayraTargetIsIOSSimulator(void) {
	return TARGET_OS_SIMULATOR;
}
*/
import "C"

func targetIsIOSSimulator() bool {
	return C.ayraTargetIsIOSSimulator() != 0
}
