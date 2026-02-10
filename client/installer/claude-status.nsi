; Claude Status Monitor - NSIS Installer Script
; Usage: makensis /DARCH=amd64 /DVERSION=x.y.z /DEXE_PATH=..\build\claude-status-amd64.exe claude-status.nsi

!ifndef ARCH
  !error "ARCH must be defined (amd64 or arm64)"
!endif

!ifndef VERSION
  !error "VERSION must be defined"
!endif

!ifndef EXE_PATH
  !error "EXE_PATH must be defined"
!endif

; ---- Includes ----
!include "MUI2.nsh"
!include "FileFunc.nsh"

; ---- General Settings ----
Name "Claude Status Monitor ${VERSION}"
OutFile "..\build\claude-status-${ARCH}-setup.exe"
InstallDir "$LOCALAPPDATA\Claude Status Monitor"
InstallDirRegKey HKCU "Software\Claude Status Monitor" "InstallDir"
RequestExecutionLevel user
Unicode True

; ---- Version Information ----
VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "Claude Status Monitor"
VIAddVersionKey "FileDescription" "Claude Status Monitor Installer (${ARCH})"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"

; ---- MUI Settings ----
!define MUI_ABORTWARNING

; ---- Pages ----
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"

; ---- Core Section (required) ----
Section "Claude Status Monitor" SecCore
  SectionIn RO

  SetOutPath "$INSTDIR"

  ; Main executable
  File "${EXE_PATH}"
  Rename "$INSTDIR\claude-status-${ARCH}.exe" "$INSTDIR\claude-status.exe"

  ; Example config
  File /oname=config.example.yaml "..\config.example.yaml"

  ; Create uninstaller
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Start Menu shortcuts
  CreateDirectory "$SMPROGRAMS\Claude Status Monitor"
  CreateShortCut "$SMPROGRAMS\Claude Status Monitor\Claude Status Monitor.lnk" \
    "$INSTDIR\claude-status.exe" "" "$INSTDIR\claude-status.exe" 0
  CreateShortCut "$SMPROGRAMS\Claude Status Monitor\Uninstall.lnk" \
    "$INSTDIR\uninstall.exe" "" "$INSTDIR\uninstall.exe" 0

  ; Registry: Add/Remove Programs
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ClaudeStatusMonitor" \
    "DisplayName" "Claude Status Monitor"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ClaudeStatusMonitor" \
    "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ClaudeStatusMonitor" \
    "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ClaudeStatusMonitor" \
    "Publisher" "Claude Status Monitor"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ClaudeStatusMonitor" \
    "DisplayIcon" '"$INSTDIR\claude-status.exe"'
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ClaudeStatusMonitor" \
    "NoModify" 1
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ClaudeStatusMonitor" \
    "NoRepair" 1

  ; Store installed size
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ClaudeStatusMonitor" \
    "EstimatedSize" $0

  ; Store install dir
  WriteRegStr HKCU "Software\Claude Status Monitor" "InstallDir" "$INSTDIR"
SectionEnd

; ---- Desktop Shortcut (optional) ----
Section "Desktop Shortcut" SecDesktop
  CreateShortCut "$DESKTOP\Claude Status Monitor.lnk" \
    "$INSTDIR\claude-status.exe" "" "$INSTDIR\claude-status.exe" 0
SectionEnd

; ---- Auto Start (optional) ----
Section "Start with Windows" SecAutoStart
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" \
    "ClaudeStatusMonitor" '"$INSTDIR\claude-status.exe" -config "$INSTDIR\config.yaml"'
SectionEnd

; ---- Section Descriptions ----
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecCore} \
    "Core application files (required)."
  !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} \
    "Create a shortcut on the Desktop."
  !insertmacro MUI_DESCRIPTION_TEXT ${SecAutoStart} \
    "Automatically start Claude Status Monitor when Windows starts."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

; ---- Uninstaller ----
Section "Uninstall"
  ; Remove auto-start registry entry
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "ClaudeStatusMonitor"

  ; Remove files
  Delete "$INSTDIR\claude-status.exe"
  Delete "$INSTDIR\config.example.yaml"
  Delete "$INSTDIR\claude-status.log"
  Delete "$INSTDIR\uninstall.exe"

  ; Remove shortcuts
  Delete "$SMPROGRAMS\Claude Status Monitor\Claude Status Monitor.lnk"
  Delete "$SMPROGRAMS\Claude Status Monitor\Uninstall.lnk"
  RMDir "$SMPROGRAMS\Claude Status Monitor"
  Delete "$DESKTOP\Claude Status Monitor.lnk"

  ; Remove install directory (only if empty, preserves user config.yaml)
  RMDir "$INSTDIR"

  ; Remove registry keys
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ClaudeStatusMonitor"
  DeleteRegKey HKCU "Software\Claude Status Monitor"
SectionEnd
