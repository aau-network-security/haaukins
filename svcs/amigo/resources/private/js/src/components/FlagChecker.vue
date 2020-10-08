<template>
    <div class="flag-form">
        <form>
            <span style="color: #444;">Flag</span>
            <div class="input-div" style="margin-top: 0px !important;">
                <div class="input-div-icon" v-bind:class="{ 'text-success': successMsg, 'text-danger': errorMsg }">
                    <i class="fa fa-flag"></i>
                </div>
                <div class="input-div-input">
                    <input class="input" type="text" :disabled="loading" @keydown="clearMessages" @click="clearMessages" v-model="flag" placeholder="HKN{**********}">
                </div>
            </div>
            <div class="text-center">
                <p v-if="errorMsg" class="text-danger">{{ errorMsg }}</p>
                <p v-if="successMsg" class="text-success">{{ successMsg }}</p>
                <p v-if="loading">
                  <span class="spinner-border" role="status" aria-hidden="true"></span>
                  <span class="pl-2">Loading next Step ...</span>
                </p>
            </div>
            <div class="row">
                <div class="col-md-6 col-12">
                  <button v-on:click="submitFlag" class="btn btn-haaukins" type="button" style="width: 100%" :disabled="loading">Verify Flag</button>
                </div>
                <div v-if="!isSkipped" class="col-md-6 mt-md-0 col-12 mt-2">
                  <button v-on:click="SkipResumeChallenge" class="btn btn-login" type="button" style="width: 100%" :disabled="loading">Skip Challenge</button>
                </div>
                <div v-else class="col-md-6 mt-md-0 col-12 mt-2">
                  <button v-on:click="SkipResumeChallenge" class="btn btn-success" type="button" style="width: 100%" :disabled="loading">Resume Challenge</button>
                </div>
            </div>
        </form>
    </div>

</template>

<script>
    /* eslint-disable */
    export default {
        name: 'FlagChecker',
        props: {
            challengeTag: String,
            isSkipped: Boolean
        },
        data: () => {
            return {
                action: 'Submit',
                description: 'Flag...',
                flag: '',
                errorMsg: '',
                successMsg: '',
                loading: false,
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
            submitFlag: async function() {
                this.loading = true
                const opts = {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ tag: this.challengeTag, flag: this.flag })
                };
                const res = await fetch('/flags/verify', opts).
                then(res => res.json());

                this.manageResponse(res, "You found a flag!")

            },
            SkipResumeChallenge: async function() {
              this.loading = true
              const opts = {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ tag: this.challengeTag, manage: this.isSkipped})
                };
                const res = await fetch('/challenge/manage', opts).
                then(res => res.json());

                this.manageResponse(res, "Challenges Status Changed")
            },
          manageResponse: function (res, output){

            this.loading = false;
            this.flag = '';
            let that = this;

            if (res.error !== undefined) {
              this.errorMsg = res.error;
              return
            }

            this.successMsg = output;
            setTimeout(function () {
              that.$bvModal.hide('challengeModal')
              that.$emit('challengeComplete')
            }, 800);

            if (res.status === "wait") {
              this.successMsg = output;
              that.$emit('stepCompleted')
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
