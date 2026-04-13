!macro NSIS_HOOK_PREINSTALL
  ; 설치 경로 선택 다이얼로그 표시
  nsDialogs::SelectFolderDialog "설치 위치를 선택하세요" $INSTDIR
  Pop $0
  ${If} $0 != "error"
    StrCpy $INSTDIR $0
  ${EndIf}
!macroend
