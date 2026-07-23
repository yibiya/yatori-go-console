"use client"

import { useState, useEffect } from "react"
import { AccountCard } from "@/components/account-card"
import { AccountDetail } from "@/components/account-detail"
import { Button } from "@/components/ui/button"
import { Plus, Users } from "lucide-react"
import { AddAccountDialog } from "@/components/add-account-dialog"
import { getAccounts, deleteAccountForUid as apiDeleteAccountForUid } from "@/api/accountApi"
import { toast } from "@/components/ui/use-toast"
import { Skeleton } from "@/components/ui/skeleton"

export type Account = {
    uid: string
    accountType: string
    url?: string
    account: string
    password: string
    isRunning: boolean
    userConfigJson: string
    status: "active" | "inactive"
    courseCount: number
    lastLogin?: string
}

export type Course = {
    courseId: string
    courseName: string
    progress: number
    totalLessons: number
    completedLessons: number
    instructor: string
}

export function AccountList() {
    const [accounts, setAccounts] = useState<Account[]>([])
    const [selectedAccount, setSelectedAccount] = useState<Account | null>(null)
    const [isAddDialogOpen, setIsAddDialogOpen] = useState(false)
    const [loading, setLoading] = useState(true)

    const fetchAccounts = async () => {
        setLoading(true)
        try {
            const response = await getAccounts()
            if (response.code === 200) {
                const formattedAccounts: Account[] = response.data.users.map((user) => ({
                    uid: user.uid,
                    accountType: user.accountType,
                    url: user.url,
                    account: user.account,
                    password: user.password,
                    isRunning: user.isRunning,
                    userConfigJson: user.userConfigJson,
                    status: "active" as const,
                    courseCount: 0,
                }))
                setAccounts(formattedAccounts)
            } else {
                console.error(response.message)
            }
        } catch (error) {
            console.error("Failed to fetch accounts:", error)
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        fetchAccounts()
    }, [])

    const handleDeleteAccount = async (uid: string) => {
        try {
            const response = await apiDeleteAccountForUid(uid)
            if (response.code === 200) {
                await fetchAccounts()
                toast({ title: "已删除", description: "账号已移除" })
            } else {
                toast({
                    title: "删除失败",
                    description: response.message,
                    variant: "destructive",
                })
            }
        } catch {
            toast({
                title: "删除失败",
                description: "网络错误，请稍后重试",
                variant: "destructive",
            })
        }
        if (selectedAccount?.uid === uid) {
            setSelectedAccount(null)
        }
    }

    if (selectedAccount) {
        return (
            <AccountDetail
                account={selectedAccount}
                onBack={() => setSelectedAccount(null)}
                onUpdated={fetchAccounts}
            />
        )
    }

    const runningCount = accounts.filter((a) => a.isRunning).length

    return (
        <div>
            <div className="mb-6 flex flex-col gap-4 sm:mb-8 sm:flex-row sm:items-end sm:justify-between">
                <div>
                    <h2 className="text-2xl font-semibold tracking-tight text-foreground">账号管理</h2>
                    <p className="mt-1 text-sm text-muted-foreground">
                        {loading
                            ? "加载中…"
                            : accounts.length === 0
                              ? "添加学习账号后开始刷课"
                              : `${accounts.length} 个账号${runningCount ? ` · ${runningCount} 个运行中` : ""}`}
                    </p>
                </div>
                <Button onClick={() => setIsAddDialogOpen(true)} className="w-full gap-2 shadow-sm sm:w-auto">
                    <Plus className="h-4 w-4" />
                    添加账号
                </Button>
            </div>

            {loading ? (
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-4 lg:grid-cols-3 xl:grid-cols-4">
                    {Array.from({ length: 4 }).map((_, i) => (
                        <div key={i} className="rounded-xl border bg-card p-4">
                            <div className="flex items-start gap-3">
                                <Skeleton className="h-11 w-11 rounded-xl" />
                                <div className="flex-1 space-y-2">
                                    <Skeleton className="h-4 w-24" />
                                    <Skeleton className="h-3 w-16" />
                                    <div className="flex gap-2 pt-1">
                                        <Skeleton className="h-5 w-14 rounded-md" />
                                        <Skeleton className="h-5 w-16 rounded-md" />
                                    </div>
                                </div>
                            </div>
                            <Skeleton className="mt-4 h-8 w-full" />
                        </div>
                    ))}
                </div>
            ) : accounts.length === 0 ? (
                <div className="flex flex-col items-center justify-center rounded-2xl border border-dashed bg-muted/30 px-4 py-16 text-center sm:py-20">
                    <div className="mb-4 rounded-2xl bg-primary/10 p-4 text-primary">
                        <Users className="h-8 w-8" />
                    </div>
                    <h3 className="text-base font-medium text-foreground sm:text-lg">还没有账号</h3>
                    <p className="mt-1 max-w-sm text-sm text-muted-foreground">
                        添加学习通 / 智慧树等平台账号，即可在这里启停刷课与查看进度
                    </p>
                    <Button onClick={() => setIsAddDialogOpen(true)} className="mt-5 gap-2">
                        <Plus className="h-4 w-4" />
                        添加第一个账号
                    </Button>
                </div>
            ) : (
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-4 lg:grid-cols-3 xl:grid-cols-4">
                    {accounts.map((account) => (
                        <AccountCard
                            key={account.uid}
                            account={account}
                            onClick={() => setSelectedAccount(account)}
                            onDelete={() => handleDeleteAccount(account.uid)}
                        />
                    ))}
                </div>
            )}

            <AddAccountDialog open={isAddDialogOpen} onOpenChange={setIsAddDialogOpen} onAdd={fetchAccounts} />
        </div>
    )
}
