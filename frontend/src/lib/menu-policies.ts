/** FrameFlow 侧栏菜单策略 obj（与 passportboot 种子一致） */
export const MENU_POLICY_OPTIONS = [
  { label: '地图空间', value: '/map' },
  { label: '实时监控', value: '/preview' },
  { label: '视频回放', value: '/recording' },
  { label: '事件核查', value: '/review' },
  { label: '报警记录', value: '/alarms' },
  { label: '联动规则', value: '/alarms/rules' },
  { label: '工作流', value: '/workflows' },
  { label: '智能搜索', value: '/search' },
  { label: '设备管理', value: '/devices' },
  { label: '模型仓库', value: '/models' },
  { label: '算法接入', value: '/algorithms' },
  { label: '通知中心', value: '/notifications' },
  { label: '用户&权限', value: '/users' },
  { label: '运维', value: '/system' },
] as const
