import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:pull_to_refresh/pull_to_refresh.dart';
import 'package:timeago/timeago.dart' as timeago;

import '../../../core/models/session_models.dart';
import '../../../core/providers/auth_provider.dart';
import '../../../core/providers/sessions_provider.dart';
import '../../../shared/theme/app_theme.dart';
import '../widgets/session_card.dart';
import '../widgets/create_session_dialog.dart';

class SessionsScreen extends ConsumerStatefulWidget {
  const SessionsScreen({super.key});

  @override
  ConsumerState<SessionsScreen> createState() => _SessionsScreenState();
}

class _SessionsScreenState extends ConsumerState<SessionsScreen> {
  final RefreshController _refreshController = RefreshController();

  @override
  void initState() {
    super.initState();
    // Load sessions on init
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(sessionsProvider.notifier).loadSessions();
    });
  }

  @override
  void dispose() {
    _refreshController.dispose();
    super.dispose();
  }

  Future<void> _onRefresh() async {
    await ref.read(sessionsProvider.notifier).refresh();
    _refreshController.refreshCompleted();
  }

  void _showCreateDialog() {
    showDialog(
      context: context,
      builder: (context) => CreateSessionDialog(
        onCreated: (session) {
          context.push('/chat/${session.id}');
        },
      ),
    );
  }

  void _openSession(Session session) {
    ref.read(sessionsProvider.notifier).selectSession(session.id);
    context.push('/chat/${session.id}');
  }

  void _openSettings() {
    context.push('/settings');
  }

  @override
  Widget build(BuildContext context) {
    final sessionsState = ref.watch(sessionsProvider);
    final user = ref.watch(currentUserProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Sessions'),
        leading: IconButton(
          icon: CircleAvatar(
            radius: 16,
            backgroundColor: AppTheme.primaryColor,
            child: Text(
              user?.username.substring(0, 1).toUpperCase() ?? 'U',
              style: const TextStyle(
                color: Colors.white,
                fontSize: 14,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
          onPressed: _openSettings,
        ),
        actions: [
          IconButton(
            icon: const Icon(Icons.add),
            onPressed: _showCreateDialog,
          ),
        ],
      ),
      body: _buildBody(sessionsState),
      floatingActionButton: FloatingActionButton(
        onPressed: _showCreateDialog,
        child: const Icon(Icons.add),
      ),
    );
  }

  Widget _buildBody(SessionsState sessionsState) {
    if (sessionsState.isLoading && sessionsState.sessions.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (sessionsState.error != null && sessionsState.sessions.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(
              Icons.error_outline,
              size: 48,
              color: AppTheme.errorColor,
            ),
            const SizedBox(height: 16),
            Text(
              'Failed to load sessions',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 8),
            TextButton(
              onPressed: () {
                ref.read(sessionsProvider.notifier).loadSessions();
              },
              child: const Text('Retry'),
            ),
          ],
        ),
      );
    }

    if (sessionsState.sessions.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.chat_bubble_outline,
              size: 64,
              color: Theme.of(context).colorScheme.onSurface.withAlpha(77),
            ),
            const SizedBox(height: 16),
            Text(
              'No sessions yet',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 8),
            Text(
              'Create a new session to get started',
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: Theme.of(context).colorScheme.onSurface.withAlpha(153),
                  ),
            ),
            const SizedBox(height: 24),
            ElevatedButton.icon(
              onPressed: _showCreateDialog,
              icon: const Icon(Icons.add),
              label: const Text('New Session'),
            ),
          ],
        ),
      );
    }

    return SmartRefresher(
      controller: _refreshController,
      onRefresh: _onRefresh,
      header: const WaterDropMaterialHeader(),
      child: ListView(
        padding: const EdgeInsets.symmetric(vertical: 8),
        children: [
          // Pinned sessions
          if (sessionsState.pinnedSessions.isNotEmpty) ...[
            _buildSectionHeader('Pinned'),
            ...sessionsState.pinnedSessions.map(
              (session) => SessionCard(
                session: session,
                onTap: () => _openSession(session),
                onPin: () => _togglePin(session),
                onArchive: () => _archiveSession(session),
                onDelete: () => _deleteSession(session),
              ),
            ),
          ],

          // Active sessions
          if (sessionsState.activeSessions.isNotEmpty) ...[
            _buildSectionHeader('Recent'),
            ...sessionsState.activeSessions.map(
              (session) => SessionCard(
                session: session,
                onTap: () => _openSession(session),
                onPin: () => _togglePin(session),
                onArchive: () => _archiveSession(session),
                onDelete: () => _deleteSession(session),
              ),
            ),
          ],

          // Archived sessions
          if (sessionsState.archivedSessions.isNotEmpty) ...[
            _buildSectionHeader('Archived'),
            ...sessionsState.archivedSessions.map(
              (session) => SessionCard(
                session: session,
                onTap: () => _openSession(session),
                onDelete: () => _deleteSession(session),
              ),
            ),
          ],

          const SizedBox(height: 80), // Space for FAB
        ],
      ),
    );
  }

  Widget _buildSectionHeader(String title) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
      child: Text(
        title,
        style: Theme.of(context).textTheme.titleSmall?.copyWith(
              color: Theme.of(context).colorScheme.onSurface.withAlpha(153),
            ),
      ),
    );
  }

  void _togglePin(Session session) {
    ref.read(sessionsProvider.notifier).updateSession(
          session.id,
          isPinned: !session.isPinned,
        );
  }

  void _archiveSession(Session session) {
    ref.read(sessionsProvider.notifier).archiveSession(session.id);
  }

  void _deleteSession(Session session) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete Session'),
        content: Text('Are you sure you want to delete "${session.title}"?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              ref.read(sessionsProvider.notifier).deleteSession(session.id);
            },
            style: TextButton.styleFrom(foregroundColor: AppTheme.errorColor),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
  }
}
