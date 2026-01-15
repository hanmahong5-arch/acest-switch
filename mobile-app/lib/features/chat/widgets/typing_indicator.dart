import 'package:flutter/material.dart';

import '../../../shared/theme/app_theme.dart';

class TypingIndicator extends StatefulWidget {
  final List<String> userIds;

  const TypingIndicator({
    super.key,
    required this.userIds,
  });

  @override
  State<TypingIndicator> createState() => _TypingIndicatorState();
}

class _TypingIndicatorState extends State<TypingIndicator>
    with SingleTickerProviderStateMixin {
  late AnimationController _controller;
  late Animation<double> _animation;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      duration: const Duration(milliseconds: 1200),
      vsync: this,
    )..repeat();

    _animation = Tween<double>(begin: 0, end: 1).animate(_controller);
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (widget.userIds.isEmpty) return const SizedBox.shrink();

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Row(
        children: [
          CircleAvatar(
            radius: 12,
            backgroundColor: AppTheme.secondaryColor.withAlpha(51),
            child: const Icon(
              Icons.smart_toy_outlined,
              size: 14,
              color: AppTheme.secondaryColor,
            ),
          ),
          const SizedBox(width: 8),
          AnimatedBuilder(
            animation: _animation,
            builder: (context, child) {
              return Row(
                children: List.generate(3, (index) {
                  final delay = index * 0.2;
                  final value = (_animation.value - delay).clamp(0.0, 1.0);
                  final scale = 0.5 + (0.5 * _bounceValue(value));

                  return Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 2),
                    child: Transform.scale(
                      scale: scale,
                      child: Container(
                        width: 8,
                        height: 8,
                        decoration: BoxDecoration(
                          color: AppTheme.secondaryColor.withAlpha(179),
                          shape: BoxShape.circle,
                        ),
                      ),
                    ),
                  );
                }),
              );
            },
          ),
          const SizedBox(width: 8),
          Text(
            widget.userIds.length == 1
                ? 'Someone is typing...'
                : '${widget.userIds.length} people are typing...',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: AppTheme.secondaryColor,
                ),
          ),
        ],
      ),
    );
  }

  double _bounceValue(double value) {
    if (value < 0.5) {
      return 4 * value * value * value;
    } else {
      return 1 - ((-2 * value + 2) * (-2 * value + 2) * (-2 * value + 2)) / 2;
    }
  }
}
