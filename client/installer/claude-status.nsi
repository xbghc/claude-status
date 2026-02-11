; Claude Status Monitor - Modern NSIS Installer Script
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
!include "nsDialogs.nsh"
!include "LogicLib.nsh"
!include "WinMessages.nsh"
!include "FileFunc.nsh"
!include "Sections.nsh"

; ---- Modern UI Colors ----
!define BG_COLOR         "FAFAF9"
!define TEXT_COLOR        "1C1917"
!define SUBTEXT_COLOR    "78716C"
!define ACCENT_COLOR     "D97706"
!define BTN_TEXT_COLOR   "FFFFFF"
!define INPUT_BG         "FFFFFF"
!define SUCCESS_COLOR    "16A34A"

; ---- Typography ----
!define FONT_NAME        "Segoe UI"
!define FONT_SIZE_TITLE  "20"
!define FONT_SIZE_NORMAL "9"
!define FONT_SIZE_SMALL  "8"
!define FONT_SIZE_BTN    "10"

; ---- General Settings ----
SetCompressor /SOLID lzma
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

; ---- Variables ----
Var hDirInput
Var hChkDesktop
Var hChkAutoStart
Var hChkLaunch

Var OptDesktop
Var OptAutoStart
Var OptLaunch

Var hFontTitle
Var hFontNormal
Var hFontSmall
Var hFontBtn
Var hFontCheck

; ---- Macro: Hide Standard NSIS Chrome ----
!macro HideStandardChrome
  ; Hide Back / Next / Cancel buttons
  GetDlgItem $0 $HWNDPARENT 1
  ShowWindow $0 ${SW_HIDE}
  GetDlgItem $0 $HWNDPARENT 2
  ShowWindow $0 ${SW_HIDE}
  GetDlgItem $0 $HWNDPARENT 3
  ShowWindow $0 ${SW_HIDE}
  ; Hide header area
  GetDlgItem $0 $HWNDPARENT 1034
  ShowWindow $0 ${SW_HIDE}
  GetDlgItem $0 $HWNDPARENT 1036
  ShowWindow $0 ${SW_HIDE}
  GetDlgItem $0 $HWNDPARENT 1037
  ShowWindow $0 ${SW_HIDE}
  GetDlgItem $0 $HWNDPARENT 1038
  ShowWindow $0 ${SW_HIDE}
  ; Hide bottom line and branding
  GetDlgItem $0 $HWNDPARENT 1256
  ShowWindow $0 ${SW_HIDE}
  GetDlgItem $0 $HWNDPARENT 1028
  ShowWindow $0 ${SW_HIDE}
!macroend

; ---- Page Declarations ----
Page custom pgInstallCreate pgInstallLeave
Page instfiles pgProgressPre pgProgressShow
Page custom pgFinishCreate pgFinishLeave

; ---- Uninstaller Pages ----
UninstPage instfiles

; ---- Language ----
LoadLanguageFile "${NSISDIR}\Contrib\Language files\SimpChinese.nlf"

; ---- UI Strings ----
!define STR_APP_NAME        "Claude Status Monitor"
!define STR_VERSION         "版本 ${VERSION}"
!define STR_INSTALL_TO      "安装位置："
!define STR_BROWSE          "浏览..."
!define STR_OPT_DESKTOP     "创建桌面快捷方式"
!define STR_OPT_AUTOSTART   "开机自动启动"
!define STR_INSTALL_BTN     "安装"
!define STR_INSTALLING      "正在安装..."
!define STR_STATUS_FILES    "正在复制文件..."
!define STR_STATUS_SHORT    "正在创建快捷方式..."
!define STR_STATUS_REG      "正在写入注册表..."
!define STR_COMPLETE        "安装完成"
!define STR_COMPLETE_MSG    "Claude Status Monitor 已成功安装。"
!define STR_LAUNCH          "启动 Claude Status Monitor"
!define STR_FINISH          "完成"
!define STR_CANCEL_CONFIRM  "确定要取消安装吗？"
!define STR_SELECT_DIR      "选择安装文件夹"
!define STR_PROC_RUNNING    "Claude Status Monitor 正在运行。$\n需要关闭它才能继续安装。"
!define STR_UNINSTALL_CONFIRM "确定要卸载 Claude Status Monitor 吗？"

; ---- Initialization ----
Function .onInit
  ; Set defaults
  StrCpy $OptDesktop "1"
  StrCpy $OptAutoStart "0"
  StrCpy $OptLaunch "1"

  ; Check if the application is already running and close it
  nsExec::ExecToStack 'tasklist /FI "IMAGENAME eq claude-status.exe" /NH'
  Pop $0  ; exit code
  Pop $1  ; output
  ${If} $0 == 0
    StrCpy $2 $1 14  ; Check first 14 chars to match "INFO: No tasks"
    ${IfNot} $2 == "INFO: No tasks"
      MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION "${STR_PROC_RUNNING}" IDOK +2
      Abort
      nsExec::ExecToLog 'taskkill /F /IM claude-status.exe'
      Sleep 1000
    ${EndIf}
  ${EndIf}

  ; Create fonts
  CreateFont $hFontTitle  "${FONT_NAME}" ${FONT_SIZE_TITLE} 600
  CreateFont $hFontNormal "${FONT_NAME}" ${FONT_SIZE_NORMAL} 400
  CreateFont $hFontSmall  "${FONT_NAME}" ${FONT_SIZE_SMALL} 400
  CreateFont $hFontBtn    "${FONT_NAME}" ${FONT_SIZE_BTN} 600
  CreateFont $hFontCheck  "${FONT_NAME}" 28 700
FunctionEnd

Function .onGUIInit
  ; Hide branding text
  GetDlgItem $0 $HWNDPARENT 1028
  ShowWindow $0 ${SW_HIDE}

  ; Hide bottom line
  GetDlgItem $0 $HWNDPARENT 1256
  ShowWindow $0 ${SW_HIDE}

  ; ---- DPI-adaptive window sizing ----
  ; Dialog unit controls scale with DPI, so pixel sizes must scale too.
  ; Base inner area: 500 x 400 at 96 DPI (100%).
  ; Get system DPI
  System::Call "user32::GetDC(p 0) p.r1"
  System::Call "gdi32::GetDeviceCaps(p r1, i 90) i.r2"  ; LOGPIXELSY = 90
  System::Call "user32::ReleaseDC(p 0, p r1)"

  ; Scale inner area: base * dpi / 96
  IntOp $3 500 * $2
  IntOp $3 $3 / 96  ; scaled inner width
  IntOp $4 400 * $2
  IntOp $4 $4 / 96  ; scaled inner height

  ; Calculate exact outer window size via AdjustWindowRectEx
  System::Call "user32::GetWindowLong(p $HWNDPARENT, i -16) i.r5"  ; GWL_STYLE
  System::Call "user32::GetWindowLong(p $HWNDPARENT, i -20) i.r6"  ; GWL_EXSTYLE
  System::Call "*(i 0, i 0, i r3, i r4) p.r7"
  System::Call "user32::AdjustWindowRectEx(p r7, i r5, i 0, i r6)"
  System::Call "*$7(i.r8, i.r9, i.R0, i.R1)"
  System::Free $7
  IntOp $R2 $R0 - $8  ; outer width
  IntOp $R3 $R1 - $9  ; outer height

  ; Center on screen
  System::Call "user32::GetSystemMetrics(i 0) i.r8"  ; SM_CXSCREEN
  System::Call "user32::GetSystemMetrics(i 1) i.r9"  ; SM_CYSCREEN
  IntOp $R4 $8 - $R2
  IntOp $R4 $R4 / 2
  IntOp $R5 $9 - $R3
  IntOp $R5 $R5 / 2

  ; Apply sizes: outer window (move + resize), inner page area (resize only)
  System::Call "user32::SetWindowPos(p $HWNDPARENT, p 0, i $R4, i $R5, i $R2, i $R3, i 0x0004)"
  GetDlgItem $0 $HWNDPARENT 1018
  System::Call "user32::SetWindowPos(p $0, p 0, i 0, i 0, i $3, i $4, i 0x0014)"
FunctionEnd

Function .onUserAbort
  MessageBox MB_YESNO|MB_ICONQUESTION "${STR_CANCEL_CONFIRM}" IDYES +2
  Abort
FunctionEnd

Function .onInstSuccess
  ; Auto-advance from instfiles to finish page
  Sleep 500
  GetDlgItem $0 $HWNDPARENT 1
  EnableWindow $0 1
  SendMessage $0 ${BM_CLICK} 0 0
FunctionEnd

; ============================================================
; Page 1: Main Install Page (Welcome + Directory + Components)
; ============================================================
Function pgInstallCreate
  !insertmacro HideStandardChrome

  nsDialogs::Create 1018
  Pop $0
  ${If} $0 == error
    Abort
  ${EndIf}
  SetCtlColors $0 ${TEXT_COLOR} ${BG_COLOR}

  ; ---- Title: App Name ----
  ${NSD_CreateLabel} 20u 14u 280u 20u "${STR_APP_NAME}"
  Pop $0
  SetCtlColors $0 ${TEXT_COLOR} ${BG_COLOR}
  SendMessage $0 ${WM_SETFONT} $hFontTitle 1

  ; ---- Version label ----
  ${NSD_CreateLabel} 20u 36u 280u 12u "${STR_VERSION}"
  Pop $0
  SetCtlColors $0 ${SUBTEXT_COLOR} ${BG_COLOR}
  SendMessage $0 ${WM_SETFONT} $hFontSmall 1

  ; ---- Horizontal separator ----
  ${NSD_CreateHLine} 20u 54u 276u 1u ""
  Pop $0

  ; ---- "Install to:" label ----
  ${NSD_CreateLabel} 20u 64u 276u 12u "${STR_INSTALL_TO}"
  Pop $0
  SetCtlColors $0 ${TEXT_COLOR} ${BG_COLOR}
  SendMessage $0 ${WM_SETFONT} $hFontNormal 1

  ; ---- Directory input field ----
  ${NSD_CreateText} 20u 78u 210u 14u "$INSTDIR"
  Pop $hDirInput
  SetCtlColors $hDirInput ${TEXT_COLOR} ${INPUT_BG}
  SendMessage $hDirInput ${WM_SETFONT} $hFontNormal 1

  ; ---- Browse button ----
  ${NSD_CreateButton} 236u 77u 60u 16u "${STR_BROWSE}"
  Pop $0
  SetCtlColors $0 ${TEXT_COLOR} ${INPUT_BG}
  SendMessage $0 ${WM_SETFONT} $hFontNormal 1
  ${NSD_OnClick} $0 pgInstallBrowse

  ; ---- Desktop shortcut checkbox ----
  ${NSD_CreateCheckbox} 20u 102u 276u 14u "${STR_OPT_DESKTOP}"
  Pop $hChkDesktop
  SetCtlColors $hChkDesktop ${TEXT_COLOR} ${BG_COLOR}
  SendMessage $hChkDesktop ${WM_SETFONT} $hFontNormal 1
  ${If} $OptDesktop == "1"
    ${NSD_Check} $hChkDesktop
  ${EndIf}

  ; ---- Auto-start checkbox ----
  ${NSD_CreateCheckbox} 20u 120u 276u 14u "${STR_OPT_AUTOSTART}"
  Pop $hChkAutoStart
  SetCtlColors $hChkAutoStart ${TEXT_COLOR} ${BG_COLOR}
  SendMessage $hChkAutoStart ${WM_SETFONT} $hFontNormal 1
  ${If} $OptAutoStart == "1"
    ${NSD_Check} $hChkAutoStart
  ${EndIf}

  ; ---- Install button (centered, accent-colored) ----
  ${NSD_CreateButton} 110u 155u 96u 26u "${STR_INSTALL_BTN}"
  Pop $0
  SetCtlColors $0 ${BTN_TEXT_COLOR} ${ACCENT_COLOR}
  SendMessage $0 ${WM_SETFONT} $hFontBtn 1
  ${NSD_OnClick} $0 pgInstallClickNext

  nsDialogs::Show
FunctionEnd

Function pgInstallBrowse
  nsDialogs::SelectFolderDialog "${STR_SELECT_DIR}" $INSTDIR
  Pop $0
  ${If} $0 != error
    StrCpy $INSTDIR $0
    ${NSD_SetText} $hDirInput $INSTDIR
  ${EndIf}
FunctionEnd

Function pgInstallClickNext
  ; Read control values
  ${NSD_GetText} $hDirInput $INSTDIR
  ${NSD_GetState} $hChkDesktop $OptDesktop
  ${NSD_GetState} $hChkAutoStart $OptAutoStart

  ; Click hidden Next button to advance
  GetDlgItem $0 $HWNDPARENT 1
  SendMessage $0 ${BM_CLICK} 0 0
FunctionEnd

Function pgInstallLeave
  ; Validate directory
  ${If} $INSTDIR == ""
    MessageBox MB_OK|MB_ICONEXCLAMATION "请选择安装目录。"
    Abort
  ${EndIf}
FunctionEnd

; ============================================================
; Page 2: Progress Page (restyled instfiles)
; ============================================================
Function pgProgressPre
  ; Enable/disable optional sections based on user choices
  ${If} $OptDesktop == ${BST_CHECKED}
    !insertmacro SelectSection ${SecDesktop}
  ${Else}
    !insertmacro UnselectSection ${SecDesktop}
  ${EndIf}

  ${If} $OptAutoStart == ${BST_CHECKED}
    !insertmacro SelectSection ${SecAutoStart}
  ${Else}
    !insertmacro UnselectSection ${SecAutoStart}
  ${EndIf}
FunctionEnd

Function pgProgressShow
  !insertmacro HideStandardChrome

  ; Get the inner page dialog
  FindWindow $R0 "#32770" "" $HWNDPARENT

  ; ---- Hide the detail listbox (scrolling log) ----
  GetDlgItem $0 $R0 1016
  ShowWindow $0 ${SW_HIDE}

  ; ---- DPI-scaled pixel positions for instfiles controls ----
  ; Base values at 96 DPI: margin=24, statusY=76, barY=100, contentW=420, barH=10, statusH=20
  System::Call "user32::GetDC(p 0) p.r1"
  System::Call "gdi32::GetDeviceCaps(p r1, i 88) i.r2"  ; LOGPIXELSX = 88
  System::Call "user32::ReleaseDC(p 0, p r1)"

  ; Scale positions: value * dpi / 96
  IntOp $3 24 * $2
  IntOp $3 $3 / 96   ; scaled margin X
  IntOp $4 420 * $2
  IntOp $4 $4 / 96   ; scaled content width
  IntOp $5 100 * $2
  IntOp $5 $5 / 96   ; scaled bar Y
  IntOp $6 10 * $2
  IntOp $6 $6 / 96   ; scaled bar height
  IntOp $7 76 * $2
  IntOp $7 $7 / 96   ; scaled status Y
  IntOp $8 20 * $2
  IntOp $8 $8 / 96   ; scaled status height

  ; ---- Restyle progress bar: make it thinner and modern ----
  GetDlgItem $0 $R0 1004
  System::Call "user32::SetWindowPos(p $0, p 0, i $3, i $5, i $4, i $6, i 0x0014)"
  ; Set progress bar color to accent amber D97706 (PBM_SETBARCOLOR = 0x0409, COLORREF = 0x00BBGGRR)
  SendMessage $0 0x0409 0 0x000677D9

  ; ---- Restyle status text ----
  GetDlgItem $0 $R0 1006
  SetCtlColors $0 ${TEXT_COLOR} ${BG_COLOR}
  SendMessage $0 ${WM_SETFONT} $hFontNormal 1
  System::Call "user32::SetWindowPos(p $0, p 0, i $3, i $7, i $4, i $8, i 0x0014)"

  ; ---- Add "Installing..." title above progress bar ----
  ; instfiles page doesn't support nsDialogs, so create a STATIC control via Win32 API
  ; Base title position at 96 DPI: (24, 30, 420, 36)
  IntOp $R1 30 * $2
  IntOp $R1 $R1 / 96  ; scaled title Y
  IntOp $R2 36 * $2
  IntOp $R2 $R2 / 96  ; scaled title height
  System::Call "user32::CreateWindowEx(i 0, t 'STATIC', t '${STR_INSTALLING}', \
    i ${WS_CHILD}|${WS_VISIBLE}, i $3, i $R1, i $4, i $R2, p $R0, p 0, p 0, p 0) p.r9"
  SetCtlColors $9 ${TEXT_COLOR} ${BG_COLOR}
  SendMessage $9 ${WM_SETFONT} $hFontTitle 1

  ; Set page background color
  SetCtlColors $R0 ${TEXT_COLOR} ${BG_COLOR}
FunctionEnd

; ============================================================
; Page 3: Finish Page
; ============================================================
Function pgFinishCreate
  !insertmacro HideStandardChrome

  nsDialogs::Create 1018
  Pop $0
  ${If} $0 == error
    Abort
  ${EndIf}
  SetCtlColors $0 ${TEXT_COLOR} ${BG_COLOR}

  ; ---- Success checkmark ----
  ${NSD_CreateLabel} 20u 20u 26u 24u "✓"
  Pop $0
  SetCtlColors $0 ${SUCCESS_COLOR} ${BG_COLOR}
  SendMessage $0 ${WM_SETFONT} $hFontCheck 1

  ; ---- "Installation Complete" title ----
  ${NSD_CreateLabel} 48u 24u 260u 18u "${STR_COMPLETE}"
  Pop $0
  SetCtlColors $0 ${TEXT_COLOR} ${BG_COLOR}
  SendMessage $0 ${WM_SETFONT} $hFontTitle 1

  ; ---- Success message ----
  ${NSD_CreateLabel} 20u 56u 280u 24u "${STR_COMPLETE_MSG}"
  Pop $0
  SetCtlColors $0 ${SUBTEXT_COLOR} ${BG_COLOR}
  SendMessage $0 ${WM_SETFONT} $hFontNormal 1

  ; ---- Launch checkbox ----
  ${NSD_CreateCheckbox} 20u 92u 280u 14u "${STR_LAUNCH}"
  Pop $hChkLaunch
  SetCtlColors $hChkLaunch ${TEXT_COLOR} ${BG_COLOR}
  SendMessage $hChkLaunch ${WM_SETFONT} $hFontNormal 1
  ${NSD_Check} $hChkLaunch

  ; ---- Finish button ----
  ${NSD_CreateButton} 110u 155u 96u 26u "${STR_FINISH}"
  Pop $0
  SetCtlColors $0 ${BTN_TEXT_COLOR} ${ACCENT_COLOR}
  SendMessage $0 ${WM_SETFONT} $hFontBtn 1
  ${NSD_OnClick} $0 pgFinishClickDone

  nsDialogs::Show
FunctionEnd

Function pgFinishClickDone
  ${NSD_GetState} $hChkLaunch $OptLaunch
  ; Click hidden Next/Finish button to close installer
  GetDlgItem $0 $HWNDPARENT 1
  SendMessage $0 ${BM_CLICK} 0 0
FunctionEnd

Function pgFinishLeave
  ${If} $OptLaunch == ${BST_CHECKED}
    Exec '"$INSTDIR\claude-status.exe"'
  ${EndIf}
FunctionEnd

; ============================================================
; Sections
; ============================================================

; ---- Core Section (required) ----
Section "Claude Status Monitor" SecCore
  SectionIn RO

  DetailPrint "${STR_STATUS_FILES}"
  SetOutPath "$INSTDIR"

  ; Main executable
  File "${EXE_PATH}"
  Rename "$INSTDIR\claude-status-${ARCH}.exe" "$INSTDIR\claude-status.exe"

  ; Example config
  File /oname=config.example.yaml "..\config.example.yaml"

  ; Create uninstaller
  WriteUninstaller "$INSTDIR\uninstall.exe"

  DetailPrint "${STR_STATUS_SHORT}"

  ; Start Menu shortcuts
  CreateDirectory "$SMPROGRAMS\Claude Status Monitor"
  CreateShortCut "$SMPROGRAMS\Claude Status Monitor\Claude Status Monitor.lnk" \
    "$INSTDIR\claude-status.exe" "" "$INSTDIR\claude-status.exe" 0
  CreateShortCut "$SMPROGRAMS\Claude Status Monitor\Uninstall.lnk" \
    "$INSTDIR\uninstall.exe" "" "$INSTDIR\uninstall.exe" 0

  DetailPrint "${STR_STATUS_REG}"

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

; ============================================================
; Uninstaller
; ============================================================
Function un.onInit
  MessageBox MB_OKCANCEL|MB_ICONQUESTION "${STR_UNINSTALL_CONFIRM}" IDOK +2
  Abort
FunctionEnd

Section "Uninstall"
  ; Close running instance before uninstalling
  nsExec::ExecToLog 'taskkill /F /IM claude-status.exe'
  Sleep 500

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
