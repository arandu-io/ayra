package io.arandu.ayra.push;

import com.google.firebase.messaging.FirebaseMessagingService;
import com.google.firebase.messaging.RemoteMessage;

/**
 * AyraMessagingService receives what the push service sends while the process
 * is alive: a new token, and a message that arrived with the application in
 * front. A notification that arrives with the application closed is drawn by
 * the system and reaches the application only when it is touched.
 */
public final class AyraMessagingService extends FirebaseMessagingService {
	@Override public void onNewToken(String token) {
		AyraPush.deliverToken(token);
	}

	@Override public void onMessageReceived(RemoteMessage message) {
		AyraPush.deliverMessage(message, false);
	}
}
