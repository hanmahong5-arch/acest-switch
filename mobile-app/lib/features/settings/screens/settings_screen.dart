import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/providers/auth_provider.dart';
import '../../../core/services/storage_service.dart';
import '../../../core/services/sync_service.dart';
import '../../../shared/theme/app_theme.dart';

class SettingsScreen extends ConsumerStatefulWidget {
  const SettingsScreen({super.key});

  @override
  ConsumerState<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends ConsumerState<SettingsScreen> {
  final _storage = StorageService.instance;
  final _sync = SyncService.instance;

  late TextEditingController _serverUrlController;
  late TextEditingController _natsUrlController;
  late TextEditingController _deviceNameController;

  bool _isEditing = false;

  @override
  void initState() {
    super.initState();
    _serverUrlController = TextEditingController(text: _storage.getServerUrl());
    _natsUrlController = TextEditingController(text: _storage.getNatsUrl());
    _deviceNameController = TextEditingController(text: _storage.getDeviceName());
  }

  @override
  void dispose() {
    _serverUrlController.dispose();
    _natsUrlController.dispose();
    _deviceNameController.dispose();
    super.dispose();
  }

  Future<void> _saveSettings() async {
    await _storage.saveServerUrl(_serverUrlController.text.trim());
    await _storage.saveNatsUrl(_natsUrlController.text.trim());
    await _storage.saveDeviceName(_deviceNameController.text.trim());

    setState(() => _isEditing = false);

    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Settings saved')),
      );
    }

    // Reconnect sync service with new settings
    _sync.disconnect();
    await _sync.connect();
  }

  void _confirmLogout() {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Sign Out'),
        content: const Text('Are you sure you want to sign out?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              ref.read(authStateProvider.notifier).logout();
            },
            style: TextButton.styleFrom(foregroundColor: AppTheme.errorColor),
            child: const Text('Sign Out'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final user = ref.watch(currentUserProvider);
    final isConnected = _sync.isConnected;

    return Scaffold(
      appBar: AppBar(
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.pop(),
        ),
        title: const Text('Settings'),
        actions: [
          if (_isEditing)
            TextButton(
              onPressed: _saveSettings,
              child: const Text('Save'),
            ),
        ],
      ),
      body: ListView(
        children: [
          // User info section
          if (user != null) ...[
            _buildSection(
              title: 'Account',
              children: [
                ListTile(
                  leading: CircleAvatar(
                    radius: 24,
                    backgroundColor: AppTheme.primaryColor,
                    child: Text(
                      user.username.substring(0, 1).toUpperCase(),
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 20,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  title: Text(
                    user.username,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                  subtitle: user.email != null
                      ? Text(user.email!)
                      : Text(
                          'Plan: ${user.plan}',
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                ),
                if (user.quotaTotal > 0)
                  Padding(
                    padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            Text(
                              'Quota',
                              style: Theme.of(context).textTheme.bodySmall,
                            ),
                            Text(
                              '\$${user.quotaUsed.toStringAsFixed(2)} / \$${user.quotaTotal.toStringAsFixed(2)}',
                              style: Theme.of(context).textTheme.bodySmall,
                            ),
                          ],
                        ),
                        const SizedBox(height: 8),
                        LinearProgressIndicator(
                          value: user.quotaUsedPercent,
                          backgroundColor: AppTheme.primaryColor.withAlpha(51),
                          valueColor: AlwaysStoppedAnimation<Color>(
                            user.quotaUsedPercent > 0.9
                                ? AppTheme.errorColor
                                : AppTheme.primaryColor,
                          ),
                        ),
                      ],
                    ),
                  ),
              ],
            ),
          ],

          // Connection status
          _buildSection(
            title: 'Connection',
            children: [
              ListTile(
                leading: Icon(
                  isConnected ? Icons.cloud_done_outlined : Icons.cloud_off_outlined,
                  color: isConnected ? AppTheme.accentColor : AppTheme.errorColor,
                ),
                title: const Text('Sync Status'),
                subtitle: Text(isConnected ? 'Connected' : 'Disconnected'),
                trailing: IconButton(
                  icon: const Icon(Icons.refresh),
                  onPressed: () async {
                    _sync.disconnect();
                    await _sync.connect();
                    setState(() {});
                  },
                ),
              ),
            ],
          ),

          // Server settings
          _buildSection(
            title: 'Server',
            children: [
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                child: TextField(
                  controller: _serverUrlController,
                  decoration: const InputDecoration(
                    labelText: 'Server URL',
                    hintText: 'http://localhost:8081',
                  ),
                  onChanged: (_) => setState(() => _isEditing = true),
                ),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                child: TextField(
                  controller: _natsUrlController,
                  decoration: const InputDecoration(
                    labelText: 'NATS WebSocket URL',
                    hintText: 'ws://localhost:8222',
                  ),
                  onChanged: (_) => setState(() => _isEditing = true),
                ),
              ),
              const SizedBox(height: 8),
            ],
          ),

          // Device settings
          _buildSection(
            title: 'Device',
            children: [
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                child: TextField(
                  controller: _deviceNameController,
                  decoration: const InputDecoration(
                    labelText: 'Device Name',
                    hintText: 'My Mobile Device',
                  ),
                  onChanged: (_) => setState(() => _isEditing = true),
                ),
              ),
              ListTile(
                leading: const Icon(Icons.fingerprint),
                title: const Text('Device ID'),
                subtitle: Text(
                  _storage.getDeviceId() ?? 'Not set',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ),
            ],
          ),

          // About section
          _buildSection(
            title: 'About',
            children: [
              ListTile(
                leading: const Icon(Icons.info_outline),
                title: const Text('Version'),
                subtitle: const Text('1.0.0'),
              ),
            ],
          ),

          // Sign out
          Padding(
            padding: const EdgeInsets.all(16),
            child: ElevatedButton(
              onPressed: _confirmLogout,
              style: ElevatedButton.styleFrom(
                backgroundColor: AppTheme.errorColor,
                foregroundColor: Colors.white,
              ),
              child: const Text('Sign Out'),
            ),
          ),

          const SizedBox(height: 32),
        ],
      ),
    );
  }

  Widget _buildSection({
    required String title,
    required List<Widget> children,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 24, 16, 8),
          child: Text(
            title.toUpperCase(),
            style: Theme.of(context).textTheme.labelMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurface.withAlpha(153),
                  letterSpacing: 0.5,
                ),
          ),
        ),
        Card(
          margin: const EdgeInsets.symmetric(horizontal: 16),
          child: Column(
            children: children,
          ),
        ),
      ],
    );
  }
}
