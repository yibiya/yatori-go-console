"use client"

import { useState, useEffect } from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { cn } from "@/lib/utils"
import { Users, FileQuestion, ChevronLeft, ChevronRight, Menu, X, LockKeyhole } from "lucide-react"

export function Sidebar() {
  const [collapsed, setCollapsed] = useState(false)
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [adminPassword, setAdminPassword] = useState("")
  const pathname = usePathname()

  // 初始化加载密码
  useEffect(() => {
    const savedPass = localStorage.getItem('yatori-admin-password') || ""
    setAdminPassword(savedPass)
  }, [])

  // 保存密码到本地
  const handlePasswordChange = (val: string) => {
    setAdminPassword(val)
    localStorage.setItem('yatori-admin-password', val)
  }

  const navItems = [
    {
      href: "/accounts",
      label: "账号管理",
      icon: Users,
    },
    {
      href: "/questions",
      label: "答题管理",
      icon: FileQuestion,
    },
  ]

  const handleMobileNavClick = () => {
    setMobileMenuOpen(false)
  }

  return (
    <>
      <button
        onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
        className="lg:hidden fixed top-4 left-4 z-50 p-2 rounded-lg bg-card border border-border shadow-lg hover:bg-accent transition-colors"
        aria-label="切换菜单"
      >
        {mobileMenuOpen ? <X className="h-6 w-6" /> : <Menu className="h-6 w-6" />}
      </button>

      {mobileMenuOpen && (
        <div
          className="lg:hidden fixed inset-0 bg-background/80 backdrop-blur-sm z-40"
          onClick={() => setMobileMenuOpen(false)}
        />
      )}

      <aside
        className={cn(
          "flex flex-col bg-sidebar border-r border-sidebar-border transition-all duration-300 ease-in-out",
          // 桌面端
          "hidden lg:flex",
          collapsed ? "lg:w-20" : "lg:w-64",
          // 移动端
          "fixed lg:relative inset-y-0 left-0 z-40",
          "w-64",
          mobileMenuOpen ? "flex" : "hidden lg:flex",
        )}
      >
        {/* Logo区域 */}
        <div className="flex items-center justify-between p-4 border-b border-sidebar-border">
          <button onClick={() => setCollapsed(!collapsed)} className="flex items-center gap-3 w-full group">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground transition-transform group-hover:scale-105">
              <span className="font-bold text-lg">Y</span>
            </div>
            {!collapsed && (
              <div className="flex-1 text-left">
                <h2 className="text-sm font-semibold text-sidebar-foreground">Yatori</h2>
                <p className="text-xs text-sidebar-foreground/60">课程管理系统</p>
              </div>
            )}
          </button>
          {!collapsed && (
            <button
              onClick={() => setCollapsed(!collapsed)}
              className="hidden lg:block p-1.5 rounded-md hover:bg-sidebar-accent text-sidebar-foreground/60 hover:text-sidebar-foreground transition-colors"
            >
              <ChevronLeft className="h-5 w-5" />
            </button>
          )}
        </div>

        {/* 展开按钮（收纳状态时显示） */}
        {collapsed && (
          <div className="hidden lg:flex justify-center p-2 border-b border-sidebar-border">
            <button
              onClick={() => setCollapsed(false)}
              className="p-1.5 rounded-md hover:bg-sidebar-accent text-sidebar-foreground/60 hover:text-sidebar-foreground transition-colors"
            >
              <ChevronRight className="h-5 w-5" />
            </button>
          </div>
        )}

        {/* 导航菜单 */}
        <nav className="flex-1 p-4 space-y-2">
          {navItems.map((item) => {
            const Icon = item.icon
            const isActive = pathname === item.href

            return (
              <Link
                key={item.href}
                href={item.href}
                onClick={handleMobileNavClick}
                className={cn(
                  "flex items-center gap-3 px-3 py-2.5 rounded-lg transition-all duration-200",
                  "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                  isActive
                    ? "bg-sidebar-primary text-sidebar-primary-foreground shadow-sm"
                    : "text-sidebar-foreground/70",
                  collapsed ? "lg:justify-center" : "",
                )}
              >
                <Icon className={cn("h-5 w-5 flex-shrink-0", isActive && "animate-in zoom-in-50 duration-200")} />
                <span
                  className={cn("font-medium text-sm animate-in fade-in-50 duration-200", collapsed && "lg:hidden")}
                >
                  {item.label}
                </span>
              </Link>
            )
          })}
        </nav>

        {/* 底部信息 */}
        <div className="p-4 border-t border-sidebar-border space-y-4">
          {!collapsed && (
            <div className="space-y-2 px-1">
              <div className="flex items-center gap-2 text-xs font-medium text-sidebar-foreground/60">
                <LockKeyhole className="h-3 w-3" />
                <span>管理权限</span>
              </div>
              <input
                type="password"
                placeholder="输入管理员密码"
                value={adminPassword}
                onChange={(e) => handlePasswordChange(e.target.value)}
                className="w-full bg-sidebar-accent/50 border border-sidebar-border rounded-md px-2 py-1.5 text-xs text-sidebar-foreground placeholder:text-sidebar-foreground/40 focus:outline-none focus:ring-1 focus:ring-sidebar-primary"
              />
            </div>
          )}

          {!collapsed ? (
            <div className="text-xs text-sidebar-foreground/50 animate-in fade-in-50 duration-200">
              <p>版本 v1.0.0</p>
              <p className="mt-1">© 2025 Yatori</p>
            </div>
          ) : (
            <div className="hidden lg:flex flex-col items-center gap-4">
              <LockKeyhole className="h-4 w-4 text-sidebar-foreground/40" />
              <div className="h-2 w-2 rounded-full bg-sidebar-primary animate-pulse"></div>
            </div>
          )}
        </div>
      </aside>
    </>
  )
}
