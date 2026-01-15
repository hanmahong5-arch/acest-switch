import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/session_models.dart';
import '../services/api_service.dart';
import '../services/sync_service.dart';

/// Sessions state notifier
class SessionsNotifier extends StateNotifier<SessionsState> {
  final ApiService _api;
  final SyncService _sync;
  StreamSubscription? _sessionUpdateSubscription;

  SessionsNotifier(this._api, this._sync) : super(SessionsState.initial()) {
    _init();
  }

  void _init() {
    // Listen for session updates from sync service
    _sessionUpdateSubscription = _sync.sessionUpdates.listen(_handleSessionUpdate);
  }

  void _handleSessionUpdate(SessionUpdate update) {
    switch (update.type) {
      case 'session_created':
        // Refresh sessions list
        refresh();
        break;
      case 'session_deleted':
        state = state.copyWith(
          sessions: state.sessions.where((s) => s.id != update.sessionId).toList(),
        );
        break;
      case 'session_update':
        final index = state.sessions.indexWhere((s) => s.id == update.sessionId);
        if (index >= 0) {
          final updatedSessions = List<Session>.from(state.sessions);
          updatedSessions[index] = updatedSessions[index].copyWith(
            title: update.title,
            isArchived: update.isArchived,
          );
          state = state.copyWith(sessions: updatedSessions);
        }
        break;
    }
  }

  Future<void> loadSessions() async {
    if (state.isLoading) return;

    state = state.copyWith(isLoading: true, error: null);

    try {
      final sessions = await _api.getSessions();
      // Sort by updated time, pinned first
      sessions.sort((a, b) {
        if (a.isPinned && !b.isPinned) return -1;
        if (!a.isPinned && b.isPinned) return 1;
        return b.updatedAt.compareTo(a.updatedAt);
      });
      state = state.copyWith(sessions: sessions, isLoading: false);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
    }
  }

  Future<void> refresh() async {
    if (state.isRefreshing) return;

    state = state.copyWith(isRefreshing: true, error: null);

    try {
      final sessions = await _api.getSessions();
      sessions.sort((a, b) {
        if (a.isPinned && !b.isPinned) return -1;
        if (!a.isPinned && b.isPinned) return 1;
        return b.updatedAt.compareTo(a.updatedAt);
      });
      state = state.copyWith(sessions: sessions, isRefreshing: false);
    } catch (e) {
      state = state.copyWith(isRefreshing: false, error: e.toString());
    }
  }

  Future<Session?> createSession(String title) async {
    try {
      final session = await _api.createSession(title);
      state = state.copyWith(
        sessions: [session, ...state.sessions],
      );
      return session;
    } catch (e) {
      state = state.copyWith(error: e.toString());
      return null;
    }
  }

  Future<void> updateSession(String sessionId, {String? title, bool? isPinned}) async {
    try {
      await _api.updateSession(sessionId, title: title, isPinned: isPinned);

      final index = state.sessions.indexWhere((s) => s.id == sessionId);
      if (index >= 0) {
        final updatedSessions = List<Session>.from(state.sessions);
        updatedSessions[index] = updatedSessions[index].copyWith(
          title: title,
          isPinned: isPinned,
        );
        state = state.copyWith(sessions: updatedSessions);
      }
    } catch (e) {
      state = state.copyWith(error: e.toString());
    }
  }

  Future<void> deleteSession(String sessionId) async {
    try {
      await _api.deleteSession(sessionId);
      state = state.copyWith(
        sessions: state.sessions.where((s) => s.id != sessionId).toList(),
      );
    } catch (e) {
      state = state.copyWith(error: e.toString());
    }
  }

  Future<void> archiveSession(String sessionId) async {
    try {
      await _api.archiveSession(sessionId);

      final index = state.sessions.indexWhere((s) => s.id == sessionId);
      if (index >= 0) {
        final updatedSessions = List<Session>.from(state.sessions);
        updatedSessions[index] = updatedSessions[index].copyWith(isArchived: true);
        state = state.copyWith(sessions: updatedSessions);
      }
    } catch (e) {
      state = state.copyWith(error: e.toString());
    }
  }

  void selectSession(String? sessionId) {
    state = state.copyWith(selectedSessionId: sessionId);
  }

  @override
  void dispose() {
    _sessionUpdateSubscription?.cancel();
    super.dispose();
  }
}

/// Sessions provider
final sessionsProvider =
    StateNotifierProvider<SessionsNotifier, SessionsState>((ref) {
  final api = ApiService.instance;
  final sync = SyncService.instance;

  return SessionsNotifier(api, sync);
});

/// Selected session provider
final selectedSessionProvider = Provider<Session?>((ref) {
  final sessionsState = ref.watch(sessionsProvider);
  final selectedId = sessionsState.selectedSessionId;
  if (selectedId == null) return null;

  return sessionsState.sessions.firstWhere(
    (s) => s.id == selectedId,
    orElse: () => Session(
      id: selectedId,
      userId: '',
      title: 'Unknown',
      createdAt: DateTime.now(),
      updatedAt: DateTime.now(),
    ),
  );
});
