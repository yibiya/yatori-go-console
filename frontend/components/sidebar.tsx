"use client"

import { useEffect, useState } from "react"
import { cn } from "@/lib/utils"
import { Users, Bot, ChevronLeft, ChevronRight, Menu, X, Lock } from "lucide-react"
import { ADMIN_PASS_KEY } from "@/api/base"

export type SidebarTab = "accounts" | "questions"

interface SidebarProps {
  activeTab: SidebarTab
  onTabChange: (tab: SidebarTab) => void
}

const navItems: { id: SidebarTab; label: string; icon: typeof Users }[] = [
  { id: "accounts", label: "账号管理", icon: Users },
  { id: "questions", label: "AI 配置", icon: Bot },
]

export function Sidebar({ activeTab, onTabChange }: SidebarProps) {
  const [collapsed, setCollapsed] = useState(false)
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [adminPass, setAdminPass] = useState("")

  useEffect(() => {
    if (typeof window !== "undefined") {
      setAdminPass(window.localStorage.getItem(ADMIN_PASS_KEY) || "")
    }
  }, [])

  const handleAdminPassChange = (value: string) => {
    setAdminPass(value)
    if (typeof window !== "undefined") {
      if (value.trim()) {
        window.localStorage.setItem(ADMIN_PASS_KEY, value)
      } else {
        window.localStorage.removeItem(ADMIN_PASS_KEY)
      }
    }
  }

  const handleMobileNavClick = () => {
    setMobileMenuOpen(false)
  }

  return (
    <>
      <button
        onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
        className="fixed left-4 top-4 z-50 rounded-xl border border-border bg-card/95 p-2.5 shadow-md backdrop-blur-sm transition-colors hover:bg-accent lg:hidden"
        aria-label="切换菜单"
      >
        {mobileMenuOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
      </button>

      {mobileMenuOpen && (
        <div
          className="lg:hidden fixed inset-0 bg-background/80 backdrop-blur-sm z-40"
          onClick={() => setMobileMenuOpen(false)}
        />
      )}

      <aside
        className={cn(
          "flex flex-col border-r border-sidebar-border bg-sidebar transition-all duration-300 ease-in-out",
          "hidden lg:flex",
          collapsed ? "lg:w-[4.5rem]" : "lg:w-60",
          "fixed inset-y-0 left-0 z-40 w-64 lg:relative",
          mobileMenuOpen ? "flex" : "hidden lg:flex",
        )}
      >
        <div className="flex items-center justify-between gap-2 border-b border-sidebar-border p-3.5">
          <button
            type="button"
            onClick={() => setCollapsed(!collapsed)}
            className="group flex min-w-0 flex-1 items-center gap-3 text-left"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-sidebar-primary text-sidebar-primary-foreground shadow-sm transition-transform group-hover:scale-105">
              <span className="text-sm font-bold">Y</span>
            </div>
            {!collapsed && (
              <div className="min-w-0 flex-1">
                <h2 className="truncate text-sm font-semibold text-sidebar-foreground">Yatori</h2>
                <p className="truncate text-[11px] text-sidebar-foreground/55">课程管理控制台</p>
              </div>
            )}
          </button>
          {!collapsed && (
            <button
              type="button"
              onClick={() => setCollapsed(!collapsed)}
              className="hidden rounded-md p-1.5 text-sidebar-foreground/55 transition-colors hover:bg-sidebar-accent hover:text-sidebar-foreground lg:block"
              aria-label="收起侧栏"
            >
              <ChevronLeft className="h-4 w-4" />
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

        <nav className="flex-1 space-y-1 p-3">
          {navItems.map((item) => {
            const Icon = item.icon
            const isActive = activeTab === item.id

            return (
              <button
                key={item.id}
                type="button"
                title={collapsed ? item.label : undefined}
                onClick={() => {
                  onTabChange(item.id)
                  handleMobileNavClick()
                }}
                className={cn(
                  "flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-colors",
                  isActive
                    ? "bg-sidebar-primary text-sidebar-primary-foreground shadow-sm"
                    : "text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                  collapsed && "lg:justify-center lg:px-0",
                )}
              >
                <Icon className="h-5 w-5 shrink-0" />
                <span className={cn(collapsed && "lg:hidden")}>{item.label}</span>
              </button>
            )
          })}
        </nav>

        <div className="space-y-3 border-t border-sidebar-border p-3">
          {!collapsed ? (
            <div className="space-y-2">
              <div className="relative">
                <Lock className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-sidebar-foreground/40" />
                <input
                  type="password"
                  value={adminPass}
                  onChange={(e) => handleAdminPassChange(e.target.value)}
                  placeholder="管理密码（可选）"
                  className={cn(
                    "w-full rounded-lg border bg-sidebar-accent/40 py-1.5 pl-8 pr-2 text-xs",
                    "border-sidebar-border text-sidebar-foreground placeholder:text-sidebar-foreground/40",
                    "focus:outline-none focus:ring-1 focus:ring-sidebar-primary",
                    adminPass && "border-emerald-500/50",
                  )}
                />
              </div>
              <p className="px-0.5 text-[11px] text-sidebar-foreground/45">© 2026 Yatori</p>
            </div>
          ) : (
            <div className="hidden flex-col items-center gap-2 lg:flex" title={adminPass ? "已设置管理密码" : "未设置管理密码"}>
              <Lock className={cn("h-4 w-4", adminPass ? "text-emerald-500" : "text-sidebar-foreground/40")} />
            </div>
          )}
        </div>
      </aside>
    </>
  )
}
