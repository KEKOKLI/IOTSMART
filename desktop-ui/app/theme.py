APP_STYLESHEET = """
QMainWindow, QWidget {
    background-color: #f8fafd;
    color: #202124;
    font-family: "Segoe UI Variable", "Segoe UI", "Google Sans";
    font-size: 13px;
}

QWidget#RootShell {
    background-color: #f8fafd;
}

QWidget#ContentShell {
    background-color: #f8fafd;
}

QScrollArea#PageScroller {
    background-color: transparent;
    border: none;
}

QScrollArea#PageScroller > QWidget > QWidget {
    background-color: #f8fafd;
}

QWidget#Sidebar {
    background-color: #ffffff;
    border-right: 1px solid #dadce0;
}

QWidget#TopBar {
    background-color: #ffffff;
    border-bottom: 1px solid #dadce0;
}

QLabel#AppTitle {
    font-size: 24px;
    font-weight: 800;
    color: #1a73e8;
}

QLabel#AppSubtitle {
    color: #5f6368;
    font-size: 12px;
}

QLabel#PageTitle {
    color: #202124;
    font-size: 24px;
    font-weight: 800;
}

QLabel#SectionTitle {
    color: #202124;
    font-size: 16px;
    font-weight: 750;
}

QLabel#HeroValue {
    font-size: 30px;
    font-weight: 800;
    color: #202124;
}

QLabel#HeroLabel {
    color: #5f6368;
    text-transform: uppercase;
    letter-spacing: 1.2px;
}

QLabel[muted="true"] {
    color: #5f6368;
}

QLabel[accent="true"] {
    color: #1a73e8;
    font-size: 15px;
    font-weight: 750;
}

QLabel[pill="true"] {
    background-color: #f1f3f4;
    color: #3c4043;
    border: 1px solid #dadce0;
    border-radius: 14px;
    padding: 6px 12px;
    font-weight: 650;
}

QLabel[pillState="ok"] {
    background-color: #e6f4ea;
    color: #137333;
    border: 1px solid #ceead6;
}

QLabel[pillState="warn"] {
    background-color: #fef7e0;
    color: #b06000;
    border: 1px solid #feefc3;
}

QLabel[pillState="bad"] {
    background-color: #fce8e6;
    color: #c5221f;
    border: 1px solid #fad2cf;
}

QFrame[hero="true"] {
    background-color: #ffffff;
    border: 1px solid #d2e3fc;
    border-radius: 18px;
}

QFrame[card="true"], QFrame[glass="true"] {
    background-color: #ffffff;
    border: 1px solid #dadce0;
    border-radius: 14px;
}

QFrame[notice="true"] {
    background-color: #fff8e1;
    border: 1px solid #feefc3;
    border-radius: 14px;
}

QListWidget {
    background-color: transparent;
    border: none;
    outline: none;
    padding: 8px;
}

QListWidget::item {
    background-color: transparent;
    border: 1px solid transparent;
    border-radius: 18px;
    margin: 4px 0;
    padding: 12px 14px;
}

QListWidget::item:hover {
    background-color: #f1f3f4;
}

QListWidget::item:selected {
    background-color: #e8f0fe;
    border: 1px solid #d2e3fc;
    color: #174ea6;
    font-weight: 750;
}

QPushButton {
    background-color: #ffffff;
    color: #1a73e8;
    border: 1px solid #dadce0;
    border-radius: 18px;
    padding: 9px 16px;
    font-weight: 700;
}

QPushButton:hover {
    background-color: #f8fafd;
    border-color: #c4c7c5;
}

QPushButton[accent="true"] {
    background-color: #1a73e8;
    color: #ffffff;
    border: 1px solid #1a73e8;
}

QPushButton[accent="true"]:hover {
    background-color: #185abc;
}

QPushButton[secondary="true"] {
    background-color: #e8f0fe;
    color: #174ea6;
    border: 1px solid #d2e3fc;
}

QPushButton[danger="true"] {
    background-color: #ffffff;
    color: #d93025;
    border: 1px solid #f4c7c3;
}

QPushButton[danger="true"]:hover {
    background-color: #fce8e6;
}

QLineEdit, QPlainTextEdit, QTextEdit, QComboBox, QSpinBox, QDoubleSpinBox, QDateTimeEdit {
    background-color: #ffffff;
    border: 1px solid #dadce0;
    border-radius: 10px;
    padding: 9px 11px;
    min-height: 26px;
    min-width: 150px;
    selection-background-color: #1a73e8;
    selection-color: #ffffff;
}

QPlainTextEdit, QTextEdit {
    min-height: 92px;
}

QLineEdit:focus, QPlainTextEdit:focus, QTextEdit:focus, QComboBox:focus, QSpinBox:focus {
    border: 1px solid #1a73e8;
}

QComboBox::drop-down {
    border: none;
    width: 26px;
}

QTableWidget, QTreeWidget {
    background-color: #ffffff;
    border: 1px solid #dadce0;
    border-radius: 12px;
    gridline-color: #eef0f1;
    alternate-background-color: #f8fafd;
}

QTableWidget::item {
    padding: 7px;
}

QHeaderView::section {
    background-color: #f1f3f4;
    color: #3c4043;
    border: none;
    border-bottom: 1px solid #dadce0;
    padding: 9px;
    font-weight: 750;
}

QSplitter::handle {
    background-color: #eef0f1;
}

QTabWidget::pane {
    border: 1px solid #dadce0;
    border-radius: 12px;
    top: -1px;
}

QTabBar::tab {
    background-color: #ffffff;
    color: #5f6368;
    border: 1px solid #dadce0;
    padding: 9px 14px;
    margin-right: 4px;
    border-top-left-radius: 10px;
    border-top-right-radius: 10px;
}

QTabBar::tab:selected {
    background-color: #e8f0fe;
    color: #174ea6;
}

QScrollBar:vertical {
    background-color: #f1f3f4;
    width: 12px;
    margin: 4px;
    border-radius: 6px;
}

QScrollBar::handle:vertical {
    background-color: #c4c7c5;
    min-height: 24px;
    border-radius: 6px;
}
"""
