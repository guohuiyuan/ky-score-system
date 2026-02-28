// static/js/main.js

document.addEventListener('DOMContentLoaded', function() {
    console.log("843考研录分系统 JS 加载完毕");

    // --- 自动计算总分逻辑 ---
    // 寻找所有带有 'calc-score' 类的输入框
    const scoreInputs = document.querySelectorAll('.calc-score'); 
    const totalScoreInput = document.querySelector('input[name="total_score"]');

    if (scoreInputs.length > 0 && totalScoreInput) {
        // 给每一个分数输入框绑定 input 事件
        scoreInputs.forEach(input => {
            input.addEventListener('input', calculateTotal);
        });
    }

    function calculateTotal() {
        let total = 0;
        scoreInputs.forEach(input => {
            const val = parseFloat(input.value);
            // 确保输入的是有效数字才进行累加
            if (!isNaN(val)) {
                total += val;
            }
        });
        // 更新总分输入框的值
        totalScoreInput.value = total;
    }
});