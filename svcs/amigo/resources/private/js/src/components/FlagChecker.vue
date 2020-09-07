<template>
    <div class="flag-form">
        <form @submit.prevent="submit">
          <span style="color: #444;">Flag</span>
          <div class="input-div" style="margin-top: 0px !important;">
              <div class="input-div-icon" v-bind:class="{ 'text-success': successMsg, 'text-danger': errorMsg }">
                  <i class="fa fa-flag"></i>
              </div>
              <div class="input-div-input">
                    <input class="input" type="text" @keydown="clearMessages" @click="clearMessages" v-model="flag" placeholder="HKN{**********}">
                </div>
            </div>
            <div class="text-center">
                <p v-if="errorMsg" class="text-danger">{{ errorMsg }}</p>
                <p v-if="successMsg" class="text-success">{{ successMsg }}</p>
            </div>
            <input type="submit" class="btn btn-login" value="Submit">
        </form>
    </div>

</template>

<script>
    /* eslint-disable */
    export default {
        name: 'FlagChecker',
        props: {
            challengeTag: String
        },
        data: () => {
            return {
                action: 'Submit',
                description: 'Flag...',
                flag: '',
                errorMsg: '',
                successMsg: '',
            }
        },
        mounted() {
            const inputs = document.querySelectorAll(".input");

            function addcl(){
                let parent = this.parentNode.parentNode;
                parent.classList.add("focus");
            }
            function remcl(){
                let parent = this.parentNode.parentNode;
                if(this.value == ""){
                    parent.classList.remove("focus");
                }
            }
            inputs.forEach(input => {
                input.addEventListener("focus", addcl);
                input.addEventListener("blur", remcl);
            });
        },
        methods: {
            clearMessages: function() {
                this.errorMsg = '';
                this.successMsg = '';
            },
            submit: async function() {
                const opts = {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ tag: this.challengeTag, flag: this.flag })
                };
                const res = await fetch('/flags/verify', opts).
                then(res => res.json());

                if (res.error !== undefined) {
                    this.errorMsg = res.error;
                    return
                }

                if (res.status === "ok") {
                    let that = this;
                    this.successMsg = "You found a flag!";
                    this.flag = '';
                    setTimeout(function () {
                        that.$bvModal.hide('challengeModal')
                        that.$emit('challengeComplete')
                    }, 800);
                }
            }
        }
    }
</script>

<style>
    .mybtn:focus{
        border-color: rgba(33, 26, 82, 0.8)!important;
        box-shadow: 0 1px 1px rgba(0, 0, 0, 0.075) inset, 0 0 8px rgba(33, 26, 82, 0.6) !important;
        outline: 0 none!important;
    }
    .flagSuccess{
        border-color: rgba(33, 26, 82, 0.8)!important;
        box-shadow: 0 1px 1px rgba(0, 0, 0, 0.075) inset, 0 0 8px rgba(33, 26, 82, 0.6) !important;
        outline: 0 none!important;
    }
    .flagError{
        border-color: rgba(220, 53, 69, 0.8)!important;
        box-shadow: 0 1px 1px rgba(0, 0, 0, 0.075) inset, 0 0 8px rgba(220, 53, 69, 0.6) !important;
        outline: 0 none!important;
    }
    .nofocus:focus{
        box-shadow: none!important;
        outline: none!important;
    }
    .icon-flag{
        position: absolute;
        margin-left: 10px;
        height: 38px;
        display: flex;
        align-items: center;
        z-index: 1000;
    }

</style>
