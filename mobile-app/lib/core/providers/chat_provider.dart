import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/message_models.dart';
import '../services/api_service.dart';
import '../services/sync_service.dart';

/// Chat state notifier for a specific session
class ChatNotifier extends StateNotifier<ChatState> {
  final String sessionId;
  final ApiService _api;
  final SyncService _sync;

  StreamSubscription? _messageSubscription;
  StreamSubscription? _typingSubscription;

  ChatNotifier(this.sessionId, this._api, this._sync)
      : super(ChatState.initial(sessionId)) {
    _init();
  }

  void _init() {
    // Subscribe to session messages
    _sync.subscribeToSession(sessionId);

    // Listen for incoming messages
    _messageSubscription = _sync.messages.listen(_handleMessage);

    // Listen for typing events
    _typingSubscription = _sync.typingEvents.listen(_handleTyping);
  }

  void _handleMessage(SyncMessage msg) {
    if (msg.sessionId != sessionId) return;

    final message = msg.toMessage();

    // Check if message already exists
    final exists = state.messages.any((m) => m.id == message.id);
    if (exists) return;

    // Add message to the list
    state = state.copyWith(
      messages: [...state.messages, message],
      isSending: false,
      isStreaming: false,
    );
  }

  void _handleTyping(TypingEvent event) {
    if (event.sessionId != sessionId) return;

    final typingUsers = Map<String, bool>.from(state.typingUsers);
    if (event.isTyping) {
      typingUsers[event.userId] = true;
    } else {
      typingUsers.remove(event.userId);
    }

    state = state.copyWith(typingUsers: typingUsers);
  }

  Future<void> loadMessages() async {
    if (state.isLoading) return;

    state = state.copyWith(isLoading: true, error: null);

    try {
      final messages = await _api.getMessages(sessionId);
      state = state.copyWith(messages: messages, isLoading: false);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
    }
  }

  Future<void> sendMessage(String content) async {
    if (content.trim().isEmpty) return;
    if (state.isSending) return;

    state = state.copyWith(isSending: true, error: null);

    try {
      // Send typing indicator (false - finished typing)
      _sync.sendTyping(sessionId, false);

      // Create message via API
      final message = await _api.createMessage(sessionId, 'user', content);

      // Add to local messages immediately
      state = state.copyWith(
        messages: [...state.messages, message],
        isSending: false,
      );
    } catch (e) {
      state = state.copyWith(isSending: false, error: e.toString());
    }
  }

  Future<void> deleteMessage(String messageId) async {
    try {
      await _api.deleteMessage(sessionId, messageId);
      state = state.copyWith(
        messages: state.messages.where((m) => m.id != messageId).toList(),
      );
    } catch (e) {
      state = state.copyWith(error: e.toString());
    }
  }

  void setTyping(bool isTyping) {
    _sync.sendTyping(sessionId, isTyping);
  }

  void clearError() {
    state = state.copyWith(error: null);
  }

  @override
  void dispose() {
    _sync.unsubscribeFromSession(sessionId);
    _messageSubscription?.cancel();
    _typingSubscription?.cancel();
    super.dispose();
  }
}

/// Chat provider family - creates a provider for each session
final chatProvider =
    StateNotifierProvider.family<ChatNotifier, ChatState, String>(
  (ref, sessionId) {
    final api = ApiService.instance;
    final sync = SyncService.instance;

    return ChatNotifier(sessionId, api, sync);
  },
);

/// Messages provider for a specific session
final messagesProvider = Provider.family<List<Message>, String>((ref, sessionId) {
  final chatState = ref.watch(chatProvider(sessionId));
  return chatState.messages;
});

/// Is sending provider for a specific session
final isSendingProvider = Provider.family<bool, String>((ref, sessionId) {
  final chatState = ref.watch(chatProvider(sessionId));
  return chatState.isSending;
});

/// Typing users provider for a specific session
final typingUsersProvider =
    Provider.family<Map<String, bool>, String>((ref, sessionId) {
  final chatState = ref.watch(chatProvider(sessionId));
  return chatState.typingUsers;
});
