package io.arandu.ayra;

import android.app.Activity;
import android.content.ClipboardManager;
import android.content.ClipData;
import android.content.Context;
import android.content.Intent;
import android.os.Handler;
import android.os.Looper;

import java.io.UnsupportedEncodingException;
import java.lang.ref.WeakReference;
import java.util.concurrent.CopyOnWriteArrayList;

public final class Ayra {
	private static final Object initLock = new Object();
	private static boolean jniLoaded;
	private static final Handler handler = new Handler(Looper.getMainLooper());

	/**
	 * init loads and initializes the Go native library and runs
	 * the Go main function.
	 *
	 * It is exported for use by Android apps that need to run Go code
	 * outside the lifecycle of the Ayra activity.
	 */
	public static synchronized void init(Context appCtx) {
		synchronized (initLock) {
			if (jniLoaded) {
				return;
			}
			String dataDir = appCtx.getFilesDir().getAbsolutePath();
			byte[] dataDirUTF8;
			try {
				dataDirUTF8 = dataDir.getBytes("UTF-8");
			} catch (UnsupportedEncodingException e) {
				throw new RuntimeException(e);
			}
			System.loadLibrary("ayra");
			runGoMain(dataDirUTF8, appCtx);
			jniLoaded = true;
		}
	}

	static private native void runGoMain(byte[] dataDir, Context context);

	/**
	 * IntentListener is told about every intent the activity receives: the one
	 * that started it and each one that brings it forward afterwards. A native
	 * capability that reads its own intents, such as a notification that was
	 * touched, registers one.
	 */
	public interface IntentListener {
		void onIntent(Intent intent);
	}

	/**
	 * PermissionListener is told the answer to a runtime permission request made
	 * through the activity.
	 */
	public interface PermissionListener {
		void onPermissionResult(int requestCode, String[] permissions, int[] grantResults);
	}

	private static final CopyOnWriteArrayList<IntentListener> intentListeners = new CopyOnWriteArrayList<IntentListener>();
	private static final CopyOnWriteArrayList<PermissionListener> permissionListeners = new CopyOnWriteArrayList<PermissionListener>();
	private static final Object intentLock = new Object();
	private static Intent pendingIntent;
	private static WeakReference<Activity> activity = new WeakReference<Activity>(null);

	/** setActivity records the activity that is in front, or null when none is. */
	static void setActivity(Activity a) {
		activity = new WeakReference<Activity>(a);
	}

	/** currentActivity answers the activity the application is shown in, or null. */
	public static Activity currentActivity() {
		return activity.get();
	}

	/**
	 * addIntentListener registers a listener. An intent that arrived before any
	 * listener existed -- the notification that started the application -- is
	 * handed to the first one that registers, once.
	 */
	public static void addIntentListener(IntentListener l) {
		Intent pending;
		synchronized (intentLock) {
			intentListeners.add(l);
			pending = pendingIntent;
			pendingIntent = null;
		}
		if (pending != null) {
			l.onIntent(pending);
		}
	}

	public static void addPermissionListener(PermissionListener l) {
		permissionListeners.add(l);
	}

	static void dispatchIntent(Intent intent) {
		if (intent == null) {
			return;
		}
		synchronized (intentLock) {
			if (intentListeners.isEmpty()) {
				pendingIntent = intent;
				return;
			}
		}
		for (IntentListener l : intentListeners) {
			l.onIntent(intent);
		}
	}

	static void dispatchPermissionResult(int requestCode, String[] permissions, int[] grantResults) {
		for (PermissionListener l : permissionListeners) {
			l.onPermissionResult(requestCode, permissions, grantResults);
		}
	}

	static void writeClipboard(Context ctx, String s) {
		ClipboardManager m = (ClipboardManager)ctx.getSystemService(Context.CLIPBOARD_SERVICE);
		m.setPrimaryClip(ClipData.newPlainText(null, s));
	}

	static String readClipboard(Context ctx) {
		ClipboardManager m = (ClipboardManager)ctx.getSystemService(Context.CLIPBOARD_SERVICE);
		ClipData c = m.getPrimaryClip();
		if (c == null || c.getItemCount() < 1) {
			return null;
		}
		return c.getItemAt(0).coerceToText(ctx).toString();
	}

	static void wakeupMainThread() {
		handler.post(new Runnable() {
			@Override public void run() {
				scheduleMainFuncs();
			}
		});
	}

	static private native void scheduleMainFuncs();
}
