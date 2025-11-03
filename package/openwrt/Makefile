include $(TOPDIR)/rules.mk

PKG_NAME:=lucicodex
PKG_VERSION:=0.1.0
PKG_RELEASE:=1

PKG_MAINTAINER:=AZ <Aezi.zhu@icloud.com>

include $(INCLUDE_DIR)/package.mk

define Package/lucicodex
  SECTION:=utils
  CATEGORY:=Utilities
  TITLE:=Natural language CLI for OpenWrt
  DEPENDS:=+libc
endef

define Package/lucicodex/description
  Natural language CLI that translates requests into safe, audited commands.
endef

define Build/Compile
        # Binary should be prebuilt via external Go toolchain; copy during install
endef

define Package/lucicodex/install
        $(INSTALL_DIR) $(1)/usr/bin
        $(INSTALL_BIN) ../../dist/lucicodex-linux-$(ARCH) $(1)/usr/bin/lucicodex || true
endef

$(eval $(call BuildPackage,lucicodex))


