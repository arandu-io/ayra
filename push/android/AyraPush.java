package io.arandu.ayra.push;

import android.app.Activity;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ApplicationInfo;
import android.content.pm.PackageManager;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;

import com.google.android.gms.tasks.OnCompleteListener;
import com.google.android.gms.tasks.Task;
import com.google.firebase.messaging.FirebaseMessaging;
import com.google.firebase.messaging.RemoteMessage;

import io.arandu.ayra.Ayra;

import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Map;

/**
 * AyraPush is the Java half of the push notifications of a native application.
 *
 * It asks for the permission, reads the registration token, and hands the Go
 * half what arrives: the token, a message received while the application is in
 * front, and the notification a person touched. It keeps nothing past the
 * process. The few messages that arrive before the Go half has attached -- the
 * service can start the process before the window exists -- wait in memory for
 * it, and are gone with the process.
 */
public final class AyraPush implements Ayra.IntentListener, Ayra.PermissionListener {
	static final int REQUEST_CODE = 0x4150;
	static final String CHANNEL_ID_KEY = "com.google.firebase.messaging.default_notification_channel_id";
	static final String CHANNEL_NAME_KEY = "io.arandu.ayra.push.channel_name";
	static final String DEFAULT_CHANNEL = "ayra_default";
	private static final int BACKLOG = 32;

	/** Permission results, as Go reads them. */
	static final int UNDETERMINED = 0;
	static final int GRANTED = 1;
	static final int DENIED = 2;

	private static final Handler main = new Handler(Looper.getMainLooper());
	private static final Object lock = new Object();
	private static final AyraPush instance = new AyraPush();
	private static boolean attached;
	private static boolean listening;
	private static boolean requesting;
	private static String lastOpened = "";
	private static final ArrayDeque<Runnable> backlog = new ArrayDeque<Runnable>();

	private AyraPush() {}

	/**
	 * attach is called by Go once its half is ready. It registers for intents,
	 * creates the notification channel and delivers what waited.
	 */
	public static void attach(Context ctx) {
		synchronized (lock) {
			if (!listening) {
				listening = true;
				Ayra.addPermissionListener(instance);
				ensureChannel(ctx);
				Ayra.addIntentListener(instance);
			}
			attached = true;
			while (!backlog.isEmpty()) {
				backlog.poll().run();
			}
		}
	}

	/**
	 * request asks for the permission to show notifications and starts the
	 * registration. The answer comes back through onPermission, once.
	 */
	public static void request(final Context ctx) {
		main.post(new Runnable() {
			@Override public void run() {
				begin(ctx);
			}
		});
	}

	private static void begin(Context ctx) {
		synchronized (lock) {
			if (requesting) {
				return;
			}
			requesting = true;
		}
		ensureChannel(ctx);

		if (Build.VERSION.SDK_INT >= 33 && ctx.checkSelfPermission("android.permission.POST_NOTIFICATIONS") != PackageManager.PERMISSION_GRANTED) {
			Activity a = Ayra.currentActivity();
			if (a == null) {
				finish(UNDETERMINED, "no activity is in front to ask the person");
				return;
			}
			a.requestPermissions(new String[] {"android.permission.POST_NOTIFICATIONS"}, REQUEST_CODE);
			return;
		}
		resolve(ctx);
	}

	@Override public void onPermissionResult(int requestCode, String[] permissions, int[] grantResults) {
		if (requestCode != REQUEST_CODE) {
			return;
		}
		Activity a = Ayra.currentActivity();
		Context ctx = a != null ? a.getApplicationContext() : null;
		boolean granted = grantResults.length > 0 && grantResults[0] == PackageManager.PERMISSION_GRANTED;
		if (!granted || ctx == null) {
			finish(DENIED, null);
			return;
		}
		resolve(ctx);
	}

	/** resolve reads the standing decision and fetches the token. */
	private static void resolve(Context ctx) {
		NotificationManager nm = (NotificationManager) ctx.getSystemService(Context.NOTIFICATION_SERVICE);
		boolean enabled = nm == null || Build.VERSION.SDK_INT < 24 || nm.areNotificationsEnabled();
		String error = null;
		try {
			FirebaseMessaging.getInstance().getToken().addOnCompleteListener(new OnCompleteListener<String>() {
				@Override public void onComplete(Task<String> task) {
					if (task.isSuccessful() && task.getResult() != null) {
						deliverToken(task.getResult());
					}
				}
			});
		} catch (RuntimeException e) {
			// No default FirebaseApp (an IllegalStateException), or a package
			// whose manifest lost the messaging component (a NullPointerException):
			// either way there is nothing to register with, and a crash in the
			// permission's callback would take the application with it.
			error = "not_configured";
		}
		finish(enabled ? GRANTED : DENIED, error);
	}

	private static void finish(int status, String error) {
		synchronized (lock) {
			requesting = false;
		}
		onPermission(status, error);
	}

	/** ensureChannel creates the notification channel the messages are drawn in. */
	static void ensureChannel(Context ctx) {
		if (Build.VERSION.SDK_INT < 26) {
			return;
		}
		NotificationManager nm = (NotificationManager) ctx.getSystemService(Context.NOTIFICATION_SERVICE);
		if (nm == null) {
			return;
		}
		String id = metaData(ctx, CHANNEL_ID_KEY, DEFAULT_CHANNEL);
		String name = metaData(ctx, CHANNEL_NAME_KEY, "Notifications");
		if (nm.getNotificationChannel(id) == null) {
			nm.createNotificationChannel(new NotificationChannel(id, name, NotificationManager.IMPORTANCE_HIGH));
		}
	}

	private static String metaData(Context ctx, String key, String fallback) {
		try {
			ApplicationInfo info = ctx.getPackageManager().getApplicationInfo(ctx.getPackageName(), PackageManager.GET_META_DATA);
			if (info.metaData != null) {
				String v = info.metaData.getString(key);
				if (v != null && !v.isEmpty()) {
					return v;
				}
			}
		} catch (PackageManager.NameNotFoundException e) {
			// The application's own package is always there.
		}
		return fallback;
	}

	// What arrives, handed to Go or held for it.

	static void deliverToken(final String token) {
		toGo(new Runnable() {
			@Override public void run() {
				onToken(token);
			}
		});
	}

	static void deliverMessage(RemoteMessage m, boolean opened) {
		String title = "";
		String body = "";
		if (m.getNotification() != null) {
			title = nz(m.getNotification().getTitle());
			body = nz(m.getNotification().getBody());
		}
		deliver(nz(m.getMessageId()), title, body, m.getData(), m.getSentTime(), opened);
	}

	private static void deliver(final String id, final String title, final String body, Map<String, String> data, final long sent, final boolean opened) {
		ArrayList<String> kv = new ArrayList<String>();
		if (data != null) {
			for (Map.Entry<String, String> e : data.entrySet()) {
				kv.add(e.getKey());
				kv.add(nz(e.getValue()));
			}
		}
		final String[] pairs = kv.toArray(new String[0]);
		toGo(new Runnable() {
			@Override public void run() {
				onMessage(id, title, body, pairs, sent, opened);
			}
		});
	}

	/** toGo runs a delivery now when Go is attached and keeps it for later when not. */
	private static void toGo(Runnable r) {
		synchronized (lock) {
			if (attached) {
				r.run();
				return;
			}
			if (backlog.size() >= BACKLOG) {
				backlog.poll();
			}
			backlog.add(r);
		}
	}

	/**
	 * onIntent reads the intent that opened or brought forward the application.
	 * A notification touched in the system tray carries the payload's data as
	 * extras, and the service's own bookkeeping under names that begin with
	 * "google." or "gcm.".
	 */
	@Override public void onIntent(Intent intent) {
		Bundle extras = intent.getExtras();
		if (extras == null) {
			return;
		}
		String id = extras.getString("google.message_id", extras.getString("message_id", ""));
		if (id.isEmpty() && !extras.containsKey("google.sent_time")) {
			return;
		}
		synchronized (lock) {
			if (!id.isEmpty() && id.equals(lastOpened)) {
				return;
			}
			lastOpened = id;
		}
		java.util.HashMap<String, String> data = new java.util.HashMap<String, String>();
		for (String key : extras.keySet()) {
			if (key.startsWith("google.") || key.startsWith("gcm.") || key.equals("from") || key.equals("collapse_key") || key.equals("message_type")) {
				continue;
			}
			Object v = extras.get(key);
			if (v != null) {
				data.put(key, v.toString());
			}
		}
		String title = extras.getString("gcm.notification.title", "");
		String body = extras.getString("gcm.notification.body", "");
		if (intent.getData() != null && !data.containsKey("link")) {
			data.put("link", intent.getData().toString());
		}
		long sent = extras.getLong("google.sent_time", 0);
		deliver(id, title, body, data, sent, true);
	}

	private static String nz(String s) {
		return s == null ? "" : s;
	}

	// The Go half. Each is called on a thread of the virtual machine, and none
	// waits for the application.

	private static native void onToken(String token);

	private static native void onMessage(String id, String title, String body, String[] data, long sent, boolean opened);

	private static native void onPermission(int status, String error);
}
