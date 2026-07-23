"use client"

import type React from "react"
import { useState } from "react"
import { Card, CardContent, CardFooter, CardHeader } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { User, Trash2, ChevronRight } from "lucide-react"
import type { Account } from "@/components/account-list"
import { getPlatformName } from "@/utils/platformUtils"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { cn } from "@/lib/utils"

type AccountCardProps = {
    account: Account
    onClick: () => void
    onDelete: () => void
}

export function AccountCard({ account, onClick, onDelete }: AccountCardProps) {
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const running = account.isRunning
    const remark = (() => {
        try {
            return JSON.parse(account.userConfigJson || "{}")?.remarkName as string | undefined
        } catch {
            return undefined
        }
    })()

    return (
        <>
            <Card
                className={cn(
                    "group cursor-pointer gap-0 py-0 transition-all duration-200",
                    "hover:border-primary/40 hover:shadow-md hover:shadow-primary/5",
                    "focus-within:ring-2 focus-within:ring-ring/40",
                    running && "border-primary/30 bg-primary/[0.02]",
                )}
                onClick={onClick}
            >
                <CardHeader className="p-4 pb-3">
                    <div className="flex items-start gap-3">
                        <div
                            className={cn(
                                "relative flex h-11 w-11 shrink-0 items-center justify-center rounded-xl",
                                running ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground",
                            )}
                        >
                            <User className="h-5 w-5" />
                            {running && (
                                <span className="absolute -right-0.5 -top-0.5 flex h-2.5 w-2.5">
                                    <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
                                    <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-emerald-500 ring-2 ring-card" />
                                </span>
                            )}
                        </div>
                        <div className="min-w-0 flex-1">
                            <h3 className="truncate font-semibold tracking-tight text-foreground" title={account.account}>
                                {remark || account.account}
                            </h3>
                            {remark ? (
                                <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground" title={account.account}>
                                    {account.account}
                                </p>
                            ) : null}
                            <div className="mt-2 flex flex-wrap items-center gap-1.5">
                                <Badge
                                    variant={running ? "default" : "secondary"}
                                    className={cn("text-[11px]", running && "bg-emerald-600 hover:bg-emerald-600")}
                                >
                                    {running ? "运行中" : "未运行"}
                                </Badge>
                                <Badge variant="outline" className="text-[11px] font-normal">
                                    {getPlatformName(account.accountType)}
                                </Badge>
                            </div>
                        </div>
                        <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 shrink-0 text-muted-foreground opacity-60 transition-opacity hover:text-destructive group-hover:opacity-100"
                            onClick={(e: React.MouseEvent) => {
                                e.stopPropagation()
                                setDeleteDialogOpen(true)
                            }}
                            aria-label="删除账号"
                        >
                            <Trash2 className="h-4 w-4" />
                        </Button>
                    </div>
                </CardHeader>

                <CardFooter className="flex items-center justify-between border-t px-4 py-2.5 text-xs text-muted-foreground">
                    <span className="truncate">点击管理课程与配置</span>
                    <ChevronRight className="h-4 w-4 shrink-0 transition-transform group-hover:translate-x-0.5" />
                </CardFooter>
            </Card>

            <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
                <DialogContent className="sm:max-w-md" onClick={(e) => e.stopPropagation()}>
                    <DialogHeader>
                        <DialogTitle className="text-destructive">确认删除账号</DialogTitle>
                        <DialogDescription>
                            确定删除 <strong className="text-foreground">{account.account}</strong>？此操作不可撤销。
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button type="button" variant="outline" onClick={() => setDeleteDialogOpen(false)}>
                            取消
                        </Button>
                        <Button
                            type="button"
                            variant="destructive"
                            onClick={() => {
                                setDeleteDialogOpen(false)
                                onDelete()
                            }}
                        >
                            删除
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </>
    )
}
