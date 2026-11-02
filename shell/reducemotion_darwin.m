#import <TargetConditionals.h>

#if TARGET_OS_IPHONE
@import UIKit;
#else
@import AppKit;
#endif

#include "reducemotion_darwin.h"

// ayraSystemReduceMotion answers the person's setting from the thread it is
// called on. It does not hop to the main queue: the frame loop calls it while the
// platform thread waits for the frame, and a call that waited for that thread
// would wait for itself.
int ayraSystemReduceMotion(void) {
#if TARGET_OS_IPHONE
	return UIAccessibilityIsReduceMotionEnabled() ? 1 : 0;
#else
	return [[NSWorkspace sharedWorkspace] accessibilityDisplayShouldReduceMotion] ? 1 : 0;
#endif
}
