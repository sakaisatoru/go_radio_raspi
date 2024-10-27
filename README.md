#go_radio_raspi
Raspberry pi で動く、mpvのフロントエンドです。go言語による開発の習作です。

##動作確認
Raspberry pi 3A+<br>
有機ELキャラクターディスプレイモジュール 16×2行 白色  SO1602AWWB-UC-WB-U (秋月電子）<br>
HT82V739使用アンプ基板　AE-82V739 ２台(秋月電子)<br>
スピーカー ダイソー３００円スピーカー

##消費電力
ボリュームを絞った状態で400mA、部屋で聞こえる程度まで大きく鳴らしてピークで700mA位流れます。raspberry pi 3A+では典型例で350mAらしく、有機ELが50〜60mA程度なので概ね妥当と思います。

##仕様
・ネットワーク（LAN）への接続設定機能がありません。Raspberry piの場合、OS書き込み時に設定可能なので省略しています。<br>

##実装待ち
・BT Speaker mode (ラジオを止めてアンプのみオンにする）はあるが、ペアリング機能がありません。別途設定操作が必要です。<br>

##既知の問題
・表示器操作モジュール名が aqm1602y となっていますが、実際は有機LEDキャラクタ表示器専用にカスタマイズしています。このままではLCDは動かせないと思うので注意してください。<br>
・0x27以外のシングルクォーテーションの類（プライム記号等）は正しく表示できません。代わりに？(0x3f)が表示されます。<br>
・表示器の都合で、アルファベットや数字、半角カナ、ギリシア文字の一部以外は表示できません。代わりに？(0x3f)が表示されます。<br>
・BT Speaker modeにて、音量調整ができません。接続している機器側での調整が必要です。<br>
・Radiko及びAFNへの接続がハードコーディングです。相手側の仕様が変わった場合は書き直しが必要です。<br>
・ロータリーエンコーダのレスポンスが悪く、バックラッシュもあります。これは消費電力の観点からポーリング頻度を抑えているためです。<br>

