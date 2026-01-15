/**
 * Import/Export Service - Phase 5.4
 * Handles configuration export and import via Deep Linking
 */

import type {
  ExportOptions,
  ConfigImportResult
} from '../../bindings/codeswitch/services/models'

// Re-export types from bindings
export type { ExportOptions, ConfigImportResult }

// Import the generated Wails bindings
import {
  ExportConfig as ExportConfigFn,
  ImportFromBase64,
  ImportFromDeepLink,
  ParseDeepLink
} from '../../bindings/codeswitch/services/importservice'

/**
 * Export current configuration as Base64-encoded string
 */
export async function exportConfig(options: ExportOptions): Promise<string> {
  return await ExportConfigFn(options)
}

/**
 * Import configuration from Base64-encoded string
 */
export async function importFromBase64(encodedConfig: string): Promise<ConfigImportResult> {
  return await ImportFromBase64(encodedConfig)
}

/**
 * Import configuration from Deep Link URL
 * Example: codeswitch://import?config=eyJ2ZXJzaW9uIjoiMS4wIiwicHJvdmlkZXJzIjpbXX0=
 */
export async function importFromDeepLink(deepLink: string): Promise<ConfigImportResult> {
  return await ImportFromDeepLink(deepLink)
}

/**
 * Parse Deep Link URL to extract the config parameter
 */
export async function parseDeepLink(deepLink: string): Promise<string> {
  return await ParseDeepLink(deepLink)
}

/**
 * Generate a shareable Deep Link URL from export options
 */
export async function generateShareLink(options: ExportOptions): Promise<string> {
  const encodedConfig = await exportConfig(options)
  return `codeswitch://import?config=${encodedConfig}`
}

/**
 * Copy text to clipboard
 */
export async function copyToClipboard(text: string): Promise<void> {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    await navigator.clipboard.writeText(text)
  } else {
    // Fallback for older browsers
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
  }
}
