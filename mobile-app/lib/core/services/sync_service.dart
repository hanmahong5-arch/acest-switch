import 'dart:async';
import 'dart:convert';

import 'package:logger/logger.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import '../models/message_models.dart';
import 'storage_service.dart';

/// Sync service for real-time message synchronization via WebSocket
class SyncService {
  SyncService._();
  static final SyncService instance = SyncService._();

  final _logger = Logger();
  final _storage = StorageService.instance;

  WebSocketChannel? _channel;
  StreamSubscription? _subscription;
  Timer? _heartbeatTimer;
  Timer? _reconnectTimer;

  bool _isConnected = false;
  int _reconnectAttempts = 0;
  static const int _maxReconnectAttempts = 10;
  static const Duration _heartbeatInterval = Duration(seconds: 30);
  static const Duration _reconnectDelay = Duration(seconds: 2);

  // Stream controllers for events
  final _connectionStateController = StreamController<ConnectionState>.broadcast();
  final _messageController = StreamController<SyncMessage>.broadcast();
  final _sessionUpdateController = StreamController<SessionUpdate>.broadcast();
  final _typingController = StreamController<TypingEvent>.broadcast();

  // Public streams
  Stream<ConnectionState> get connectionState => _connectionStateController.stream;
  Stream<SyncMessage> get messages => _messageController.stream;
  Stream<SessionUpdate> get sessionUpdates => _sessionUpdateController.stream;
  Stream<TypingEvent> get typingEvents => _typingController.stream;

  bool get isConnected => _isConnected;

  /// Connect to the sync server
  Future<void> connect() async {
    if (_isConnected) return;

    final natsUrl = _storage.getNatsUrl();
    final token = await _storage.getAccessToken();
    final userId = _storage.getUserId();

    if (token == null || userId == null) {
      _logger.w('Cannot connect: missing token or user ID');
      return;
    }

    try {
      _logger.i('Connecting to sync server: $natsUrl');
      _connectionStateController.add(ConnectionState.connecting);

      // Build WebSocket URL with auth
      final wsUrl = Uri.parse(natsUrl).replace(
        queryParameters: {'token': token, 'user_id': userId},
      );

      _channel = WebSocketChannel.connect(wsUrl);

      _subscription = _channel!.stream.listen(
        _handleMessage,
        onError: _handleError,
        onDone: _handleDisconnect,
      );

      _isConnected = true;
      _reconnectAttempts = 0;
      _connectionStateController.add(ConnectionState.connected);
      _startHeartbeat();

      _logger.i('Connected to sync server');
    } catch (e) {
      _logger.e('Failed to connect: $e');
      _connectionStateController.add(ConnectionState.error);
      _scheduleReconnect();
    }
  }

  /// Disconnect from the sync server
  void disconnect() {
    _stopHeartbeat();
    _reconnectTimer?.cancel();
    _subscription?.cancel();
    _channel?.sink.close();
    _channel = null;
    _isConnected = false;
    _connectionStateController.add(ConnectionState.disconnected);
    _logger.i('Disconnected from sync server');
  }

  /// Send a message through WebSocket
  void send(Map<String, dynamic> data) {
    if (!_isConnected || _channel == null) {
      _logger.w('Cannot send: not connected');
      return;
    }

    try {
      _channel!.sink.add(jsonEncode(data));
    } catch (e) {
      _logger.e('Failed to send message: $e');
    }
  }

  /// Subscribe to a session for real-time updates
  void subscribeToSession(String sessionId) {
    send({
      'type': 'subscribe',
      'subject': 'chat.*.${sessionId}.msg',
    });
  }

  /// Unsubscribe from a session
  void unsubscribeFromSession(String sessionId) {
    send({
      'type': 'unsubscribe',
      'subject': 'chat.*.${sessionId}.msg',
    });
  }

  /// Send typing indicator
  void sendTyping(String sessionId, bool isTyping) {
    send({
      'type': 'typing',
      'session_id': sessionId,
      'is_typing': isTyping,
    });
  }

  void _handleMessage(dynamic data) {
    try {
      final json = jsonDecode(data as String) as Map<String, dynamic>;
      final type = json['type'] as String?;

      switch (type) {
        case 'message':
        case 'user_message':
        case 'ai_message':
          _messageController.add(SyncMessage.fromJson(json));
          break;

        case 'session_update':
        case 'session_created':
        case 'session_deleted':
          _sessionUpdateController.add(SessionUpdate.fromJson(json));
          break;

        case 'typing':
          _typingController.add(TypingEvent.fromJson(json));
          break;

        case 'pong':
          // Heartbeat response, ignore
          break;

        default:
          _logger.d('Unknown message type: $type');
      }
    } catch (e) {
      _logger.e('Failed to parse message: $e');
    }
  }

  void _handleError(Object error) {
    _logger.e('WebSocket error: $error');
    _connectionStateController.add(ConnectionState.error);
    _handleDisconnect();
  }

  void _handleDisconnect() {
    _isConnected = false;
    _stopHeartbeat();
    _connectionStateController.add(ConnectionState.disconnected);
    _scheduleReconnect();
  }

  void _startHeartbeat() {
    _stopHeartbeat();
    _heartbeatTimer = Timer.periodic(_heartbeatInterval, (_) {
      send({'type': 'ping'});
    });
  }

  void _stopHeartbeat() {
    _heartbeatTimer?.cancel();
    _heartbeatTimer = null;
  }

  void _scheduleReconnect() {
    if (_reconnectAttempts >= _maxReconnectAttempts) {
      _logger.w('Max reconnect attempts reached');
      _connectionStateController.add(ConnectionState.failed);
      return;
    }

    final delay = _reconnectDelay * (_reconnectAttempts + 1);
    _logger.i('Scheduling reconnect in ${delay.inSeconds}s (attempt ${_reconnectAttempts + 1})');

    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(delay, () {
      _reconnectAttempts++;
      connect();
    });
  }

  void dispose() {
    disconnect();
    _connectionStateController.close();
    _messageController.close();
    _sessionUpdateController.close();
    _typingController.close();
  }
}

/// Connection state enum
enum ConnectionState {
  disconnected,
  connecting,
  connected,
  error,
  failed,
}

/// Sync message model
class SyncMessage {
  final String id;
  final String type;
  final String sessionId;
  final String userId;
  final String role;
  final String content;
  final String? model;
  final int? tokensInput;
  final int? tokensOutput;
  final double? cost;
  final DateTime timestamp;

  SyncMessage({
    required this.id,
    required this.type,
    required this.sessionId,
    required this.userId,
    required this.role,
    required this.content,
    this.model,
    this.tokensInput,
    this.tokensOutput,
    this.cost,
    required this.timestamp,
  });

  factory SyncMessage.fromJson(Map<String, dynamic> json) {
    return SyncMessage(
      id: json['id'] ?? '',
      type: json['type'] ?? 'message',
      sessionId: json['session_id'] ?? '',
      userId: json['user_id'] ?? '',
      role: json['role'] ?? 'user',
      content: json['content'] ?? '',
      model: json['model'],
      tokensInput: json['tokens']?['input'],
      tokensOutput: json['tokens']?['output'],
      cost: (json['cost'] as num?)?.toDouble(),
      timestamp: json['timestamp'] != null
          ? DateTime.fromMillisecondsSinceEpoch(json['timestamp'])
          : DateTime.now(),
    );
  }

  Message toMessage() {
    return Message(
      id: id,
      sessionId: sessionId,
      userId: userId,
      role: role,
      content: content,
      model: model,
      tokensInput: tokensInput ?? 0,
      tokensOutput: tokensOutput ?? 0,
      tokensReasoning: 0,
      cost: cost ?? 0,
      createdAt: timestamp,
    );
  }
}

/// Session update event
class SessionUpdate {
  final String type;
  final String sessionId;
  final String? title;
  final bool? isArchived;
  final DateTime timestamp;

  SessionUpdate({
    required this.type,
    required this.sessionId,
    this.title,
    this.isArchived,
    required this.timestamp,
  });

  factory SessionUpdate.fromJson(Map<String, dynamic> json) {
    return SessionUpdate(
      type: json['type'] ?? 'session_update',
      sessionId: json['session_id'] ?? '',
      title: json['title'],
      isArchived: json['is_archived'],
      timestamp: json['timestamp'] != null
          ? DateTime.fromMillisecondsSinceEpoch(json['timestamp'])
          : DateTime.now(),
    );
  }
}

/// Typing event
class TypingEvent {
  final String sessionId;
  final String userId;
  final String deviceId;
  final bool isTyping;

  TypingEvent({
    required this.sessionId,
    required this.userId,
    required this.deviceId,
    required this.isTyping,
  });

  factory TypingEvent.fromJson(Map<String, dynamic> json) {
    return TypingEvent(
      sessionId: json['session_id'] ?? '',
      userId: json['user_id'] ?? '',
      deviceId: json['device_id'] ?? '',
      isTyping: json['is_typing'] ?? false,
    );
  }
}
