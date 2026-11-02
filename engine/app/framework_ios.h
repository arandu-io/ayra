#pragma once
#include <UIKit/UIKit.h>

@interface AyraViewController : UIViewController
// barsLight says the ground behind the status bar is light, so that its content
// is drawn dark.
@property (nonatomic) BOOL barsLight;
@end
