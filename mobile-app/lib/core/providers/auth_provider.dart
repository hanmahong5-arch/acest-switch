import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';

import '../models/auth_models.dart';
import '../services/api_service.dart';
import '../services/storage_service.dart';
import '../services/sync_service.dart';

/// Auth state notifier
class AuthStateNotifier extends StateNotifier<AsyncValue<AuthState>> {
  final StorageService _storage;
  final ApiService _api;
  final SyncService _sync;

  AuthStateNotifier(this._storage, this._api, this._sync)
      : super(const AsyncValue.loading()) {
    _init();
  }

  Future<void> _init() async {
    try {
      final token = await _storage.getAccessToken();
      if (token != null) {
        // Try to get current user
        final user = await _api.getCurrentUser();
        state = AsyncValue.data(AuthState.authenticated(user));

        // Connect to sync service
        await _sync.connect();
      } else {
        state = AsyncValue.data(AuthState.initial());
      }
    } catch (e) {
      // Token invalid or expired
      await _storage.clearTokens();
      state = AsyncValue.data(AuthState.initial());
    }
  }

  Future<void> login(String username, String password) async {
    state = AsyncValue.data(AuthState.loading());

    try {
      // Generate device ID if not exists
      var deviceId = _storage.getDeviceId();
      if (deviceId == null) {
        deviceId = const Uuid().v4();
        await _storage.saveDeviceId(deviceId);
      }

      final request = LoginRequest(
        username: username,
        password: password,
        deviceId: deviceId,
        deviceName: _storage.getDeviceName(),
      );

      final response = await _api.login(request);

      // Save tokens and user info
      await _storage.saveAccessToken(response.accessToken);
      await _storage.saveRefreshToken(response.refreshToken);
      await _storage.saveUserId(response.user.id);

      state = AsyncValue.data(AuthState.authenticated(response.user));

      // Connect to sync service
      await _sync.connect();
    } catch (e) {
      state = AsyncValue.data(AuthState.error(e.toString()));
    }
  }

  Future<void> logout() async {
    try {
      await _api.logout();
    } catch (e) {
      // Ignore errors during logout
    }

    _sync.disconnect();
    await _storage.clearTokens();
    state = AsyncValue.data(AuthState.initial());
  }

  Future<void> refreshUser() async {
    try {
      final user = await _api.getCurrentUser();
      state = AsyncValue.data(AuthState.authenticated(user));
    } catch (e) {
      // Ignore errors
    }
  }
}

/// Auth state provider
final authStateProvider =
    StateNotifierProvider<AuthStateNotifier, AsyncValue<AuthState>>((ref) {
  final storage = StorageService.instance;
  final api = ApiService.instance..init();
  final sync = SyncService.instance;

  return AuthStateNotifier(storage, api, sync);
});

/// Current user provider
final currentUserProvider = Provider<UserInfo?>((ref) {
  final authState = ref.watch(authStateProvider);
  return authState.valueOrNull?.user;
});

/// Is logged in provider
final isLoggedInProvider = Provider<bool>((ref) {
  final authState = ref.watch(authStateProvider);
  return authState.valueOrNull?.isLoggedIn ?? false;
});
