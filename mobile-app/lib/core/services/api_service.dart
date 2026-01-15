import 'package:dio/dio.dart';
import 'package:logger/logger.dart';

import '../models/auth_models.dart';
import '../models/session_models.dart';
import '../models/message_models.dart';
import 'storage_service.dart';

/// API service for communicating with Sync Service
class ApiService {
  ApiService._();
  static final ApiService instance = ApiService._();

  late Dio _dio;
  final _logger = Logger();
  final _storage = StorageService.instance;

  bool _initialized = false;

  void init() {
    if (_initialized) return;

    _dio = Dio(BaseOptions(
      connectTimeout: const Duration(seconds: 30),
      receiveTimeout: const Duration(seconds: 60),
      sendTimeout: const Duration(seconds: 30),
    ));

    // Add interceptors
    _dio.interceptors.add(InterceptorsWrapper(
      onRequest: (options, handler) async {
        // Add base URL
        options.baseUrl = _storage.getServerUrl();

        // Add auth token
        final token = await _storage.getAccessToken();
        if (token != null) {
          options.headers['Authorization'] = 'Bearer $token';
        }

        _logger.d('Request: ${options.method} ${options.path}');
        handler.next(options);
      },
      onResponse: (response, handler) {
        _logger.d('Response: ${response.statusCode} ${response.requestOptions.path}');
        handler.next(response);
      },
      onError: (error, handler) async {
        _logger.e('Error: ${error.message}');

        // Handle 401 - try to refresh token
        if (error.response?.statusCode == 401) {
          try {
            final refreshed = await _refreshToken();
            if (refreshed) {
              // Retry the request
              final opts = error.requestOptions;
              final token = await _storage.getAccessToken();
              opts.headers['Authorization'] = 'Bearer $token';
              final response = await _dio.fetch(opts);
              return handler.resolve(response);
            }
          } catch (e) {
            _logger.e('Token refresh failed: $e');
          }
        }

        handler.next(error);
      },
    ));

    _initialized = true;
  }

  Future<bool> _refreshToken() async {
    final refreshToken = await _storage.getRefreshToken();
    if (refreshToken == null) return false;

    try {
      final response = await _dio.post(
        '/api/v1/auth/refresh',
        data: {'refresh_token': refreshToken},
      );

      final data = response.data;
      await _storage.saveAccessToken(data['access_token']);
      if (data['refresh_token'] != null) {
        await _storage.saveRefreshToken(data['refresh_token']);
      }
      return true;
    } catch (e) {
      return false;
    }
  }

  // ===== Auth APIs =====

  Future<AuthResponse> login(LoginRequest request) async {
    final response = await _dio.post(
      '/api/v1/auth/login',
      data: request.toJson(),
    );
    return AuthResponse.fromJson(response.data);
  }

  Future<void> logout() async {
    await _dio.post('/api/v1/auth/logout');
    await _storage.clearTokens();
  }

  Future<UserInfo> getCurrentUser() async {
    final response = await _dio.get('/api/v1/user/me');
    return UserInfo.fromJson(response.data);
  }

  // ===== Session APIs =====

  Future<List<Session>> getSessions() async {
    final response = await _dio.get('/api/v1/sessions');
    final sessions = response.data['sessions'] as List<dynamic>;
    return sessions.map((e) => Session.fromJson(e)).toList();
  }

  Future<Session> createSession(String title) async {
    final response = await _dio.post(
      '/api/v1/sessions',
      data: {'title': title},
    );
    return Session.fromJson(response.data);
  }

  Future<Session> getSession(String sessionId) async {
    final response = await _dio.get('/api/v1/sessions/$sessionId');
    return Session.fromJson(response.data);
  }

  Future<void> updateSession(String sessionId, {String? title, bool? isPinned}) async {
    await _dio.put(
      '/api/v1/sessions/$sessionId',
      data: {
        if (title != null) 'title': title,
        if (isPinned != null) 'is_pinned': isPinned,
      },
    );
  }

  Future<void> deleteSession(String sessionId) async {
    await _dio.delete('/api/v1/sessions/$sessionId');
  }

  Future<void> archiveSession(String sessionId) async {
    await _dio.post('/api/v1/sessions/$sessionId/archive');
  }

  // ===== Message APIs =====

  Future<List<Message>> getMessages(String sessionId, {int limit = 100, int offset = 0}) async {
    final response = await _dio.get(
      '/api/v1/sessions/$sessionId/messages',
      queryParameters: {'limit': limit, 'offset': offset},
    );
    final messages = response.data['messages'] as List<dynamic>;
    return messages.map((e) => Message.fromJson(e)).toList();
  }

  Future<Message> createMessage(String sessionId, String role, String content) async {
    final response = await _dio.post(
      '/api/v1/sessions/$sessionId/messages',
      data: {'role': role, 'content': content},
    );
    return Message.fromJson(response.data);
  }

  Future<void> deleteMessage(String sessionId, String messageId) async {
    await _dio.delete('/api/v1/sessions/$sessionId/messages/$messageId');
  }

  // ===== Sync APIs =====

  Future<SyncResponse> syncData({String? lastMsgId}) async {
    final response = await _dio.post(
      '/api/v1/sync',
      data: {
        if (lastMsgId != null) 'last_msg_id': lastMsgId,
      },
    );
    return SyncResponse.fromJson(response.data);
  }

  // ===== Presence APIs =====

  Future<void> sendHeartbeat() async {
    await _dio.post(
      '/api/v1/heartbeat',
      data: {
        'device_type': 'mobile',
        'client_version': '1.0.0',
      },
    );
  }

  Future<void> sendTypingEvent(String sessionId, bool isTyping) async {
    await _dio.post(
      '/api/v1/sessions/$sessionId/typing',
      data: {'is_typing': isTyping},
    );
  }
}

/// Sync response model
class SyncResponse {
  final List<Session> sessions;
  final List<Message> messages;
  final int serverTime;
  final bool hasMore;

  SyncResponse({
    required this.sessions,
    required this.messages,
    required this.serverTime,
    required this.hasMore,
  });

  factory SyncResponse.fromJson(Map<String, dynamic> json) {
    return SyncResponse(
      sessions: (json['sessions'] as List<dynamic>?)
              ?.map((e) => Session.fromJson(e))
              .toList() ??
          [],
      messages: (json['messages'] as List<dynamic>?)
              ?.map((e) => Message.fromJson(e))
              .toList() ??
          [],
      serverTime: json['server_time'] ?? 0,
      hasMore: json['has_more'] ?? false,
    );
  }
}
