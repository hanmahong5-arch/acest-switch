/// Message model
class Message {
  final String id;
  final String sessionId;
  final String userId;
  final String role;
  final String content;
  final String? model;
  final int tokensInput;
  final int tokensOutput;
  final int tokensReasoning;
  final double cost;
  final Map<String, dynamic>? metadata;
  final DateTime createdAt;

  Message({
    required this.id,
    required this.sessionId,
    required this.userId,
    required this.role,
    required this.content,
    this.model,
    this.tokensInput = 0,
    this.tokensOutput = 0,
    this.tokensReasoning = 0,
    this.cost = 0,
    this.metadata,
    required this.createdAt,
  });

  factory Message.fromJson(Map<String, dynamic> json) {
    return Message(
      id: json['id'] ?? '',
      sessionId: json['session_id'] ?? '',
      userId: json['user_id'] ?? '',
      role: json['role'] ?? 'user',
      content: json['content'] ?? '',
      model: json['model'],
      tokensInput: json['tokens_input'] ?? 0,
      tokensOutput: json['tokens_output'] ?? 0,
      tokensReasoning: json['tokens_reasoning'] ?? 0,
      cost: (json['cost'] as num?)?.toDouble() ?? 0,
      metadata: json['metadata'],
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'])
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'session_id': sessionId,
        'user_id': userId,
        'role': role,
        'content': content,
        'model': model,
        'tokens_input': tokensInput,
        'tokens_output': tokensOutput,
        'tokens_reasoning': tokensReasoning,
        'cost': cost,
        'metadata': metadata,
        'created_at': createdAt.toIso8601String(),
      };

  bool get isUser => role == 'user';
  bool get isAssistant => role == 'assistant';
  bool get isSystem => role == 'system';

  int get totalTokens => tokensInput + tokensOutput + tokensReasoning;
}

/// Message role enum
enum MessageRole {
  user,
  assistant,
  system,
}

extension MessageRoleExtension on MessageRole {
  String get value {
    switch (this) {
      case MessageRole.user:
        return 'user';
      case MessageRole.assistant:
        return 'assistant';
      case MessageRole.system:
        return 'system';
    }
  }

  static MessageRole fromString(String value) {
    switch (value) {
      case 'user':
        return MessageRole.user;
      case 'assistant':
        return MessageRole.assistant;
      case 'system':
        return MessageRole.system;
      default:
        return MessageRole.user;
    }
  }
}

/// Chat state for a session
class ChatState {
  final String sessionId;
  final List<Message> messages;
  final bool isLoading;
  final bool isSending;
  final bool isStreaming;
  final String? error;
  final String? pendingMessage;
  final Map<String, bool> typingUsers;

  ChatState({
    required this.sessionId,
    this.messages = const [],
    this.isLoading = false,
    this.isSending = false,
    this.isStreaming = false,
    this.error,
    this.pendingMessage,
    this.typingUsers = const {},
  });

  ChatState copyWith({
    String? sessionId,
    List<Message>? messages,
    bool? isLoading,
    bool? isSending,
    bool? isStreaming,
    String? error,
    String? pendingMessage,
    Map<String, bool>? typingUsers,
  }) {
    return ChatState(
      sessionId: sessionId ?? this.sessionId,
      messages: messages ?? this.messages,
      isLoading: isLoading ?? this.isLoading,
      isSending: isSending ?? this.isSending,
      isStreaming: isStreaming ?? this.isStreaming,
      error: error,
      pendingMessage: pendingMessage ?? this.pendingMessage,
      typingUsers: typingUsers ?? this.typingUsers,
    );
  }

  factory ChatState.initial(String sessionId) => ChatState(sessionId: sessionId);

  factory ChatState.loading(String sessionId) =>
      ChatState(sessionId: sessionId, isLoading: true);
}

/// Typing status for display
class TypingStatus {
  final String userId;
  final String? username;
  final String deviceId;
  final DateTime timestamp;

  TypingStatus({
    required this.userId,
    this.username,
    required this.deviceId,
    required this.timestamp,
  });

  bool get isExpired =>
      DateTime.now().difference(timestamp) > const Duration(seconds: 5);
}
