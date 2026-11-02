@import UIKit;
@import UserNotifications;

#include <stdlib.h>
#include <string.h>
#include "_cgo_export.h"

// The iOS half of the push notifications: the permission, the registration with
// APNs, and the two moments a notification reaches the application -- while it
// is in front, and when it was touched.

// flatten turns a payload into a list of alternating keys and values, the shape
// the Go half reads. aps and the service's own bookkeeping are left out; a
// nested value becomes its JSON.
static void ayraPushDeliver(NSDictionary *info, NSString *identifier, NSString *title, NSString *body, int opened) {
	NSMutableArray<NSString *> *pairs = [NSMutableArray array];
	for (id key in info) {
		if (![key isKindOfClass:[NSString class]]) {
			continue;
		}
		NSString *name = (NSString *)key;
		if ([name isEqualToString:@"aps"] || [name hasPrefix:@"gcm."] || [name hasPrefix:@"google."]) {
			continue;
		}
		id value = info[key];
		NSString *text = nil;
		if ([value isKindOfClass:[NSString class]]) {
			text = value;
		} else if ([value isKindOfClass:[NSNumber class]]) {
			text = [value stringValue];
		} else if ([NSJSONSerialization isValidJSONObject:value]) {
			NSData *json = [NSJSONSerialization dataWithJSONObject:value options:0 error:nil];
			text = [[NSString alloc] initWithData:json encoding:NSUTF8StringEncoding];
		}
		if (text != nil) {
			[pairs addObject:name];
			[pairs addObject:text];
		}
	}
	NSString *messageID = info[@"gcm.message_id"];
	if (![messageID isKindOfClass:[NSString class]]) {
		messageID = identifier;
	}

	int n = (int)pairs.count;
	char **kv = calloc(n > 0 ? n : 1, sizeof(char *));
	for (int i = 0; i < n; i++) {
		kv[i] = strdup(pairs[i].UTF8String);
	}
	char *cid = strdup(messageID.UTF8String ?: "");
	char *ctitle = strdup(title.UTF8String ?: "");
	char *cbody = strdup(body.UTF8String ?: "");
	ayraPushMessage(cid, ctitle, cbody, kv, n, opened);
	for (int i = 0; i < n; i++) {
		free(kv[i]);
	}
	free(kv);
	free(cid);
	free(ctitle);
	free(cbody);
}

@interface AyraPushCenter : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation AyraPushCenter

// A notification that arrives with the application in front draws nothing by
// itself. It is handed to the application, which decides whether to show it.
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
	   willPresentNotification:(UNNotification *)notification
		 withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
	UNNotificationContent *c = notification.request.content;
	ayraPushDeliver(c.userInfo, notification.request.identifier, c.title, c.body, 0);
	completionHandler(UNNotificationPresentationOptionNone);
}

// A notification the person touched.
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
didReceiveNotificationResponse:(UNNotificationResponse *)response
		 withCompletionHandler:(void (^)(void))completionHandler {
	UNNotificationContent *c = response.notification.request.content;
	ayraPushDeliver(c.userInfo, response.notification.request.identifier, c.title, c.body, 1);
	completionHandler();
}

@end

static AyraPushCenter *center;

// ayra_push_install sets the delegate. It has to be set before the application
// finishes launching for the notification that started it to be delivered, which
// is why the application delegate calls ayra_push_launched, and it is idempotent
// for the calls that follow.
static void install(void) {
	if (center == nil) {
		center = [[AyraPushCenter alloc] init];
	}
	[UNUserNotificationCenter currentNotificationCenter].delegate = center;
}

void ayra_push_launched(void) {
	install();
}

void ayra_push_ensure(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		install();
	});
}

void ayra_push_request(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		install();
		UNUserNotificationCenter *c = [UNUserNotificationCenter currentNotificationCenter];
		UNAuthorizationOptions options = UNAuthorizationOptionAlert | UNAuthorizationOptionBadge | UNAuthorizationOptionSound;
		[c requestAuthorizationWithOptions:options completionHandler:^(BOOL granted, NSError *error) {
			ayraPushPermission(granted ? 1 : 2);
			if (granted) {
				dispatch_async(dispatch_get_main_queue(), ^{
					[[UIApplication sharedApplication] registerForRemoteNotifications];
				});
			}
		}];
	});
}

void ayra_push_registered(const void *token, unsigned long length) {
	const unsigned char *bytes = token;
	char *hex = malloc(length * 2 + 1);
	for (unsigned long i = 0; i < length; i++) {
		sprintf(hex + i * 2, "%02x", bytes[i]);
	}
	hex[length * 2] = 0;
	ayraPushToken(hex);
	free(hex);
}

// A failed registration leaves the application with no token, which is what the
// person who asked for one sees; the reason goes to the log.
void ayra_push_failed(const char *reason) {
	NSLog(@"ayra push: registering with the push service failed: %s", reason);
}

void ayra_push_remote(CFTypeRef userInfo) {
	NSDictionary *info = (__bridge NSDictionary *)userInfo;
	NSDictionary *aps = info[@"aps"];
	NSDictionary *alert = [aps[@"alert"] isKindOfClass:[NSDictionary class]] ? aps[@"alert"] : nil;
	NSString *title = alert[@"title"] ?: @"";
	NSString *body = alert[@"body"] ?: ([aps[@"alert"] isKindOfClass:[NSString class]] ? aps[@"alert"] : @"");
	ayraPushDeliver(info, @"", title, body, 0);
}
