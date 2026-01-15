import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:hive_flutter/hive_flutter.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Storage service for managing local data persistence
class StorageService {
  StorageService._();
  static final StorageService instance = StorageService._();

  late SharedPreferences _prefs;
  late FlutterSecureStorage _secureStorage;
  late Box<dynamic> _settingsBox;

  bool _initialized = false;

  Future<void> init() async {
    if (_initialized) return;

    _prefs = await SharedPreferences.getInstance();
    _secureStorage = const FlutterSecureStorage(
      aOptions: AndroidOptions(encryptedSharedPreferences: true),
      iOptions: IOSOptions(accessibility: KeychainAccessibility.first_unlock),
    );
    _settingsBox = await Hive.openBox('settings');

    _initialized = true;
  }

  // ===== Secure Storage (for tokens) =====

  Future<void> saveAccessToken(String token) async {
    await _secureStorage.write(key: 'access_token', value: token);
  }

  Future<String?> getAccessToken() async {
    return await _secureStorage.read(key: 'access_token');
  }

  Future<void> saveRefreshToken(String token) async {
    await _secureStorage.write(key: 'refresh_token', value: token);
  }

  Future<String?> getRefreshToken() async {
    return await _secureStorage.read(key: 'refresh_token');
  }

  Future<void> clearTokens() async {
    await _secureStorage.delete(key: 'access_token');
    await _secureStorage.delete(key: 'refresh_token');
  }

  // ===== Shared Preferences (for settings) =====

  Future<void> saveServerUrl(String url) async {
    await _prefs.setString('server_url', url);
  }

  String getServerUrl() {
    return _prefs.getString('server_url') ?? 'http://localhost:8081';
  }

  Future<void> saveNatsUrl(String url) async {
    await _prefs.setString('nats_url', url);
  }

  String getNatsUrl() {
    return _prefs.getString('nats_url') ?? 'ws://localhost:8222';
  }

  Future<void> saveUserId(String userId) async {
    await _prefs.setString('user_id', userId);
  }

  String? getUserId() {
    return _prefs.getString('user_id');
  }

  Future<void> saveDeviceId(String deviceId) async {
    await _prefs.setString('device_id', deviceId);
  }

  String? getDeviceId() {
    return _prefs.getString('device_id');
  }

  Future<void> saveDeviceName(String name) async {
    await _prefs.setString('device_name', name);
  }

  String getDeviceName() {
    return _prefs.getString('device_name') ?? 'Mobile Device';
  }

  // ===== Hive Box (for complex settings) =====

  Future<void> saveSetting(String key, dynamic value) async {
    await _settingsBox.put(key, value);
  }

  T? getSetting<T>(String key, {T? defaultValue}) {
    return _settingsBox.get(key, defaultValue: defaultValue) as T?;
  }

  // ===== Clear all data =====

  Future<void> clearAll() async {
    await clearTokens();
    await _prefs.clear();
    await _settingsBox.clear();
  }
}
