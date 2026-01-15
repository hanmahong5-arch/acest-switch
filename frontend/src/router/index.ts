import { createRouter, createWebHashHistory } from 'vue-router'
import MainPage from '../components/Main/Index.vue'
import LogsPage from '../components/Logs/Index.vue'
import GeneralPage from '../components/General/Index.vue'
import McpPage from '../components/Mcp/index.vue'
import SkillPage from '../components/Skill/Index.vue'
import SyncPage from '../components/Sync/Index.vue'

const routes = [
  { path: '/', component: MainPage },
  { path: '/logs', component: LogsPage },
  { path: '/settings', component: GeneralPage },
  { path: '/mcp', component: McpPage },
  { path: '/skill', component: SkillPage },
  { path: '/sync', component: SyncPage },
  { path: '/gateway', component: () => import('../components/Gateway/Index.vue') },
  // Admin routes
  {
    path: '/admin',
    component: () => import('../components/Admin/Index.vue'),
    children: [
      { path: '', redirect: '/admin/dashboard' },
      { path: 'dashboard', component: () => import('../components/Admin/Dashboard.vue') },
      { path: 'users', component: () => import('../components/Admin/Users/Index.vue') },
      { path: 'sessions', component: () => import('../components/Admin/Sessions/Index.vue') },
      { path: 'stats', component: () => import('../components/Admin/Stats/Index.vue') },
      { path: 'audit', component: () => import('../components/Admin/Audit/Index.vue') },
      { path: 'alerts', component: () => import('../components/Admin/Alerts/Index.vue') },
      // Billing routes
      { path: 'billing', redirect: '/admin/billing/subscriptions' },
      { path: 'billing/subscriptions', component: () => import('../components/Admin/Billing/Subscriptions/Index.vue') },
      { path: 'billing/balances', component: () => import('../components/Admin/Billing/Balances/Index.vue') },
      { path: 'billing/payments', component: () => import('../components/Admin/Billing/Payments/Index.vue') },
      { path: 'billing/settings', component: () => import('../components/Admin/Billing/Settings/Index.vue') },
    ]
  },
]

export default createRouter({
  history: createWebHashHistory(), // Use createWebHashHistory for hash-based routing
  routes
})
