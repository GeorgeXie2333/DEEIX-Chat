"use client"

import * as React from "react"
import { useRouter } from "next/navigation"

import { useSidebar } from "@/components/ui/sidebar"

export function useMobileSidebarNavigation() {
  const router = useRouter()
  const { isMobile } = useSidebar()

  return React.useCallback((href: string, event: React.MouseEvent<HTMLAnchorElement>) => {
    if (!isMobile) {
      return
    }

    event.preventDefault()
    router.push(href)
  }, [isMobile, router])
}
