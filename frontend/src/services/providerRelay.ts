import { Call } from '@wailsio/runtime'

const serviceName = 'codeswitch/services.ProviderRelayService'

export const isRoundRobinEnabled = async (): Promise<boolean> => {
  return Call.ByName(`${serviceName}.IsRoundRobinEnabled`)
}

export const setRoundRobinEnabled = async (enabled: boolean): Promise<void> => {
  await Call.ByName(`${serviceName}.SetRoundRobinEnabled`, enabled)
}
