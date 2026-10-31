/*
Package notifications implements permissions to show notifications and to
receive them from a push service.

# Android

The following entries will be added to AndroidManifest.xml:

	<uses-permission android:name="android.permission.POST_NOTIFICATIONS"/>

POST_NOTIFICATIONS is a "dangerous" permission from Android 13: the application
asks the person for it at run time, and github.com/arandu-io/ayra/push does that
in its Request. See documentation for package
github.com/arandu-io/ayra/engine/app/permission for more information.

# iOS

The application's Info.plist gains the remote-notification background mode, and
the provisioning profile it is signed with has to carry the aps-environment
entitlement, which the packager checks before it signs.
*/
package notifications
