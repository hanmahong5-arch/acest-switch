/// Login request model
class LoginRequest {
  final String username;
  final String password;
  final String deviceId;
  final String deviceName;
  final String deviceType;

  LoginRequest({
    required this.username,
    required this.password,
    required this.deviceId,
    required this.deviceName,
    this.deviceType = 'mobile',
  });

  Map<String, dynamic> toJson() => {
        'username': username,
        'password': password,
        'device_id': deviceId,
        'device_name': deviceName,
        'device_type': deviceType,
      };
}

/// Auth response model
class AuthResponse {
  final String accessToken;
  final String refreshToken;
  final int expiresIn;
  final UserInfo user;

  AuthResponse({
    required this.accessToken,
    required this.refreshToken,
    required this.expiresIn,
    required this.user,
  });

  factory AuthResponse.fromJson(Map<String, dynamic> json) {
    return AuthResponse(
      accessToken: json['access_token'] ?? '',
      refreshToken: json['refresh_token'] ?? '',
      expiresIn: json['expires_in'] ?? 3600,
      user: UserInfo.fromJson(json['user'] ?? {}),
    );
  }
}

/// User info model
class UserInfo {
  final String id;
  final String username;
  final String? email;
  final String? avatarUrl;
  final String plan;
  final double quotaTotal;
  final double quotaUsed;
  final bool isAdmin;
  final DateTime createdAt;

  UserInfo({
    required this.id,
    required this.username,
    this.email,
    this.avatarUrl,
    this.plan = 'free',
    this.quotaTotal = 0,
    this.quotaUsed = 0,
    this.isAdmin = false,
    required this.createdAt,
  });

  factory UserInfo.fromJson(Map<String, dynamic> json) {
    return UserInfo(
      id: json['id'] ?? '',
      username: json['username'] ?? '',
      email: json['email'],
      avatarUrl: json['avatar_url'],
      plan: json['plan'] ?? 'free',
      quotaTotal: (json['quota_total'] as num?)?.toDouble() ?? 0,
      quotaUsed: (json['quota_used'] as num?)?.toDouble() ?? 0,
      isAdmin: json['is_admin'] ?? false,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'])
          : DateTime.now(),
    );
  }

  double get quotaRemaining => quotaTotal - quotaUsed;
  double get quotaUsedPercent => quotaTotal > 0 ? quotaUsed / quotaTotal : 0;
}

/// Auth state for state management
class AuthState {
  final bool isLoggedIn;
  final bool isLoading;
  final String? error;
  final UserInfo? user;

  AuthState({
    this.isLoggedIn = false,
    this.isLoading = false,
    this.error,
    this.user,
  });

  AuthState copyWith({
    bool? isLoggedIn,
    bool? isLoading,
    String? error,
    UserInfo? user,
  }) {
    return AuthState(
      isLoggedIn: isLoggedIn ?? this.isLoggedIn,
      isLoading: isLoading ?? this.isLoading,
      error: error,
      user: user ?? this.user,
    );
  }

  factory AuthState.initial() => AuthState();

  factory AuthState.loading() => AuthState(isLoading: true);

  factory AuthState.authenticated(UserInfo user) => AuthState(
        isLoggedIn: true,
        user: user,
      );

  factory AuthState.error(String message) => AuthState(error: message);
}
