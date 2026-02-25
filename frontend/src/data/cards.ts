export type AutomationCard = {
  id: number
  name: string
  apiUrl: string
  apiKey: string
  officialSite: string
  icon: string
  tint: string
  accent: string
  enabled: boolean
  // 模型白名单：声明 provider 支持的模型（精确或通配符）
  supportedModels?: Record<string, boolean>
  // 模型映射：external model -> internal model
  modelMapping?: Record<string, string>
  // 优先级分组：1-10，数字越小优先级越高
  level?: number
}

export const automationCardGroups: Record<'claude' | 'codex' | 'gemini-cli' | 'picoclaw', AutomationCard[]> = {
  claude: [
    {
      id: 99,
      name: 'Anthropic',
      apiUrl: 'https://api.anthropic.com',
      apiKey: '',
      officialSite: 'https://console.anthropic.com',
      icon: 'anthropic',
      tint: 'rgba(204, 119, 85, 0.16)',
      accent: '#cc7755',
      enabled: false,
      supportedModels: {
        'claude-opus-4-*': true,
        'claude-sonnet-4-*': true,
        'claude-3-*': true,
        'claude-*': true,
      },
    },
    {
      id: 100,
      name: '0011',
      apiUrl: 'https://0011.ai',
      apiKey: '',
      officialSite: 'https://0011.ai',
      icon: 'aicoding',
      tint: 'rgba(10, 132, 255, 0.14)',
      accent: '#0aff5cff',
      enabled: false,
    },
    {
      id: 101,
      name: 'AICoding.sh',
      apiUrl: 'https://api.aicoding.sh',
      apiKey: '',
      officialSite: 'https://aicoding.sh',
      icon: 'aicoding',
      tint: 'rgba(10, 132, 255, 0.14)',
      accent: '#0a84ff',
      enabled: false,
    },
    {
      id: 102,
      name: 'Kimi',
      apiUrl: 'https://api.moonshot.cn/anthropic',
      apiKey: '',
      officialSite: 'https://kimi.moonshot.cn',
      icon: 'kimi',
      tint: 'rgba(16, 185, 129, 0.16)',
      accent: '#10b981',
      enabled: false,
    },
    {
      id: 103,
      name: 'Deepseek',
      apiUrl: 'https://api.deepseek.com/anthropic',
      apiKey: '',
      officialSite: 'https://www.deepseek.com',
      icon: 'deepseek',
      tint: 'rgba(251, 146, 60, 0.18)',
      accent: '#f97316',
      enabled: false,
    },
  ],
  codex: [
    {
      id: 201,
      name: 'AICoding.sh',
      apiUrl: 'https://api.aicoding.sh',
      apiKey: '',
      officialSite: 'https://www.aicoding.sh',
      icon: 'aicoding',
      tint: 'rgba(236, 72, 153, 0.16)',
      accent: '#ec4899',
      enabled: false,
    },
  ],
  'gemini-cli': [
    {
      id: 301,
      name: 'Google Gemini',
      apiUrl: 'https://generativelanguage.googleapis.com/v1beta/openai',
      apiKey: '',
      officialSite: 'https://aistudio.google.com',
      icon: 'google-color',
      tint: 'rgba(66, 133, 244, 0.16)',
      accent: '#4285f4',
      enabled: false,
      supportedModels: {
        'gemini-*': true,
        'gemini-1.5-flash': true,
        'gemini-1.5-pro': true,
        'gemini-2.0-flash-exp': true,
        'gemini-2.0-flash-thinking-exp': true,
        'gemini-3-flash-preview': true,
      },
      level: 1,
    },
  ],
  picoclaw: [
    {
      id: 401,
      name: 'OpenAI',
      apiUrl: 'https://api.openai.com',
      apiKey: '',
      officialSite: 'https://platform.openai.com',
      icon: 'openai',
      tint: 'rgba(16, 163, 127, 0.14)',
      accent: '#10a37f',
      enabled: false,
    },
    {
      id: 402,
      name: 'OpenRouter',
      apiUrl: 'https://openrouter.ai/api',
      apiKey: '',
      officialSite: 'https://openrouter.ai',
      icon: 'openrouter',
      tint: 'rgba(139, 92, 246, 0.14)',
      accent: '#8b5cf6',
      enabled: false,
    },
  ],
}

export function createAutomationCards(data: AutomationCard[] = []): AutomationCard[] {
  return data.map((item) => ({
    ...item,
    officialSite: item.officialSite ?? '',
  }))
}
