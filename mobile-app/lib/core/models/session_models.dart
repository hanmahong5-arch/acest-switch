/// Session model
class Session {
  final String id;
  final String userId;
  final String title;
  final String? model;
  final int messageCount;
  final int tokenCount;
  final double cost;
  final bool isPinned;
  final bool isArchived;
  final DateTime createdAt;
  final DateTime updatedAt;

  Session({
    required this.id,
    required this.userId,
    required this.title,
    this.model,
    this.messageCount = 0,
    this.tokenCount = 0,
    this.cost = 0,
    this.isPinned = false,
    this.isArchived = false,
    required this.createdAt,
    required this.updatedAt,
  });

  factory Session.fromJson(Map<String, dynamic> json) {
    return Session(
      id: json['id'] ?? '',
      userId: json['user_id'] ?? '',
      title: json['title'] ?? 'Untitled',
      model: json['model'],
      messageCount: json['message_count'] ?? 0,
      tokenCount: json['token_count'] ?? 0,
      cost: (json['cost'] as num?)?.toDouble() ?? 0,
      isPinned: json['is_pinned'] ?? false,
      isArchived: json['is_archived'] ?? false,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'])
          : DateTime.now(),
      updatedAt: json['updated_at'] != null
          ? DateTime.parse(json['updated_at'])
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'user_id': userId,
        'title': title,
        'model': model,
        'message_count': messageCount,
        'token_count': tokenCount,
        'cost': cost,
        'is_pinned': isPinned,
        'is_archived': isArchived,
        'created_at': createdAt.toIso8601String(),
        'updated_at': updatedAt.toIso8601String(),
      };

  Session copyWith({
    String? id,
    String? userId,
    String? title,
    String? model,
    int? messageCount,
    int? tokenCount,
    double? cost,
    bool? isPinned,
    bool? isArchived,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) {
    return Session(
      id: id ?? this.id,
      userId: userId ?? this.userId,
      title: title ?? this.title,
      model: model ?? this.model,
      messageCount: messageCount ?? this.messageCount,
      tokenCount: tokenCount ?? this.tokenCount,
      cost: cost ?? this.cost,
      isPinned: isPinned ?? this.isPinned,
      isArchived: isArchived ?? this.isArchived,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }
}

/// Sessions list state
class SessionsState {
  final List<Session> sessions;
  final bool isLoading;
  final bool isRefreshing;
  final String? error;
  final String? selectedSessionId;

  SessionsState({
    this.sessions = const [],
    this.isLoading = false,
    this.isRefreshing = false,
    this.error,
    this.selectedSessionId,
  });

  SessionsState copyWith({
    List<Session>? sessions,
    bool? isLoading,
    bool? isRefreshing,
    String? error,
    String? selectedSessionId,
  }) {
    return SessionsState(
      sessions: sessions ?? this.sessions,
      isLoading: isLoading ?? this.isLoading,
      isRefreshing: isRefreshing ?? this.isRefreshing,
      error: error,
      selectedSessionId: selectedSessionId ?? this.selectedSessionId,
    );
  }

  List<Session> get pinnedSessions =>
      sessions.where((s) => s.isPinned && !s.isArchived).toList();

  List<Session> get activeSessions =>
      sessions.where((s) => !s.isPinned && !s.isArchived).toList();

  List<Session> get archivedSessions =>
      sessions.where((s) => s.isArchived).toList();

  factory SessionsState.initial() => SessionsState();

  factory SessionsState.loading() => SessionsState(isLoading: true);
}
