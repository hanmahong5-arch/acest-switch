import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:timeago/timeago.dart' as timeago;

import '../../../core/models/message_models.dart';
import '../../../shared/theme/app_theme.dart';

class MessageBubble extends StatelessWidget {
  final Message message;
  final bool isFirstInGroup;
  final bool isLastInGroup;
  final VoidCallback? onDelete;

  const MessageBubble({
    super.key,
    required this.message,
    this.isFirstInGroup = true,
    this.isLastInGroup = true,
    this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    final isUser = message.isUser;
    final theme = Theme.of(context);

    return Padding(
      padding: EdgeInsets.only(
        top: isFirstInGroup ? 8 : 2,
        bottom: isLastInGroup ? 8 : 2,
      ),
      child: Row(
        mainAxisAlignment:
            isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          if (!isUser && isLastInGroup) ...[
            CircleAvatar(
              radius: 14,
              backgroundColor: AppTheme.secondaryColor,
              child: const Icon(
                Icons.smart_toy_outlined,
                size: 16,
                color: Colors.white,
              ),
            ),
            const SizedBox(width: 8),
          ] else if (!isUser) ...[
            const SizedBox(width: 36),
          ],
          Flexible(
            child: GestureDetector(
              onLongPress: () => _showMessageMenu(context),
              child: Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 14,
                  vertical: 10,
                ),
                decoration: BoxDecoration(
                  color: isUser
                      ? AppTheme.primaryColor
                      : theme.cardTheme.color ?? theme.colorScheme.surface,
                  borderRadius: BorderRadius.only(
                    topLeft: Radius.circular(isUser || !isFirstInGroup ? 16 : 4),
                    topRight: Radius.circular(!isUser || !isFirstInGroup ? 16 : 4),
                    bottomLeft: Radius.circular(isUser || !isLastInGroup ? 16 : 4),
                    bottomRight: Radius.circular(!isUser || !isLastInGroup ? 16 : 4),
                  ),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Message content
                    if (message.isAssistant)
                      MarkdownBody(
                        data: message.content,
                        styleSheet: MarkdownStyleSheet(
                          p: TextStyle(
                            color: theme.colorScheme.onSurface,
                            fontSize: 15,
                          ),
                          code: TextStyle(
                            backgroundColor: theme.colorScheme.surface,
                            fontFamily: 'monospace',
                            fontSize: 13,
                          ),
                          codeblockDecoration: BoxDecoration(
                            color: theme.colorScheme.surface,
                            borderRadius: BorderRadius.circular(8),
                          ),
                        ),
                        selectable: true,
                      )
                    else
                      Text(
                        message.content,
                        style: TextStyle(
                          color: isUser ? Colors.white : theme.colorScheme.onSurface,
                          fontSize: 15,
                        ),
                      ),

                    // Metadata
                    if (isLastInGroup) ...[
                      const SizedBox(height: 4),
                      Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            timeago.format(message.createdAt, locale: 'en_short'),
                            style: TextStyle(
                              color: isUser
                                  ? Colors.white.withAlpha(179)
                                  : theme.colorScheme.onSurface.withAlpha(153),
                              fontSize: 11,
                            ),
                          ),
                          if (message.totalTokens > 0) ...[
                            const SizedBox(width: 8),
                            Text(
                              '${message.totalTokens} tokens',
                              style: TextStyle(
                                color: isUser
                                    ? Colors.white.withAlpha(179)
                                    : theme.colorScheme.onSurface.withAlpha(153),
                                fontSize: 11,
                              ),
                            ),
                          ],
                        ],
                      ),
                    ],
                  ],
                ),
              ),
            ),
          ),
          if (isUser && isLastInGroup) ...[
            const SizedBox(width: 8),
            CircleAvatar(
              radius: 14,
              backgroundColor: AppTheme.primaryColor,
              child: const Icon(
                Icons.person_outline,
                size: 16,
                color: Colors.white,
              ),
            ),
          ] else if (isUser) ...[
            const SizedBox(width: 36),
          ],
        ],
      ),
    );
  }

  void _showMessageMenu(BuildContext context) {
    showModalBottomSheet(
      context: context,
      builder: (context) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.copy_outlined),
              title: const Text('Copy'),
              onTap: () {
                Clipboard.setData(ClipboardData(text: message.content));
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: Text('Copied to clipboard')),
                );
              },
            ),
            if (onDelete != null)
              ListTile(
                leading: Icon(
                  Icons.delete_outline,
                  color: AppTheme.errorColor,
                ),
                title: Text(
                  'Delete',
                  style: TextStyle(color: AppTheme.errorColor),
                ),
                onTap: () {
                  Navigator.pop(context);
                  onDelete?.call();
                },
              ),
          ],
        ),
      ),
    );
  }
}
