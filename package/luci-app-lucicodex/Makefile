include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-lucicodex
PKG_VERSION:=0.3.0
PKG_RELEASE:=1

LUCI_TITLE:=LuciCodex - Natural Language CLI for OpenWrt (Web UI)
LUCI_PKGARCH:=all
LUCI_DEPENDS:=+luci-base +luci-compat +lucicodex
LUCI_MAINTAINER:=AZ <Aezi.zhu@icloud.com>
LUCI_DESCRIPTION:=LuCI UI for LuciCodex, the secure natural-language CLI for OpenWrt.

include $(TOPDIR)/feeds/luci/luci.mk

# call BuildPackage - OpenWrt build system will pick this up for LuCI
$(eval $(call BuildPackage,$(PKG_NAME)))


