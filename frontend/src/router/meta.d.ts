import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    // When true, the route requires a valid admin token.
    requiresAdmin?: boolean
    // When true, the route requires a valid portal user token.
    requiresUser?: boolean
    // Hide the route from server-side title (default: false).
    hideInTitle?: boolean
    // Login screen role selected by clean route aliases such as /admin/login.
    authRole?: 'admin' | 'portal'
    // Page title (i18n key resolved by useRouteTitle composable in later groups).
    titleKey?: string
  }
}
