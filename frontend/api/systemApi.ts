import apiClient from './base'

export interface BasicSetting {
  completionTone?: number
  colorLog?: number
  logOutFileSw?: number
  logLevel?: string
  logModel?: number
  webModel?: number
  adminPassword?: string
}

export interface AiSetting {
  aiType: string
  aiUrl: string
  model: string
  API_KEY: string
}

export interface ApiQueSetting {
  url: string
}

export interface EmailInform {
  sw: number
  smtpHost: string
  smtpPort: number
  userName: string
  password?: string
}

export interface SystemConfig {
  basicSetting: BasicSetting
  emailInform: EmailInform
  aiSetting: AiSetting
  apiQueSetting: ApiQueSetting
}

export interface SystemConfigResponse {
  code: number
  message: string
  data: SystemConfig
}

export const getSystemConfig = async (): Promise<SystemConfigResponse> => {
  try {
    return await apiClient.get('/v1/getGlobalSetting') as SystemConfigResponse
  } catch (error: any) {
    return {
      code: 500,
      message: error.response?.data?.message || '获取系统配置失败',
      data: {} as SystemConfig
    }
  }
}

export const updateSystemConfig = async (config: SystemConfig): Promise<{ code: number; message: string }> => {
  try {
    return await apiClient.post('/v1/updateGlobalSetting', config) as { code: number; message: string }
  } catch (error: any) {
    return {
      code: 500,
      message: error.response?.data?.message || '更新系统配置失败'
    }
  }
}
