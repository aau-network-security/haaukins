<template>
    <form @submit.prevent="submit">
<!--        <div class="field has-addons">-->
<!--            <div class="control has-icons-left is-expanded">-->
<!--                <input class="input" v-bind:class="{ 'is-success': successMsg, 'is-danger': errorMsg }" type="text" @keydown="clearMessages" @click="clearMessages" v-model="flag" :placeholder="description">-->
<!--                <span class="icon is-small is-left" v-bind:class="{ 'has-text-success': successMsg, 'has-text-danger': errorMsg }">-->
<!--                    <i class="fas fa-flag"></i>-->
<!--                </span>-->
<!--            </div>-->
<!--            <div class="control">-->
<!--                <button type="submit" class="button is-info">-->
<!--                    {{ action }}-->
<!--                </button>-->
<!--            </div>-->
<!--        </div>-->
<!--        <div class="field">-->
<!--            <p v-if="errorMsg" class="help is-danger">{{ errorMsg }}</p>-->
<!--            <p v-if="successMsg" class="help is-success">{{ successMsg }}</p>-->
<!--        </div>-->
        <div class="input-group mb-3">
            <span class="icon-flag" v-bind:class="{ 'text-success': successMsg, 'text-danger': errorMsg }">
                <i class="fa fa-flag" aria-hidden="true"></i>
            </span>
            <input type="text" class="form-control mybtn" placeholder="Flag..." aria-describedby="button-flag" @keydown="clearMessages" @click="clearMessages" v-model="flag" v-bind:class="{ 'flagSuccess': successMsg, 'flagError': errorMsg }" style="padding-left: 35px;">
            <div class="input-group-append">
                <button class="btn btn-haaukins nofocus" type="submit" id="button-flag">Submit</button>
            </div>
        </div>
        <div class="text-center">
            <p v-if="errorMsg" class="text-danger">{{ errorMsg }}</p>
            <p v-if="successMsg" class="text-success">{{ successMsg }}</p>
        </div>
    </form>
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
                window.console.log({ tag: this.challengeTag, flag: this.flag });
                const res = await fetch('/flags/verify', opts).
                then(res => res.json());

                if (res.error !== undefined) {
                    this.errorMsg = res.error;
                    return
                }

                if (res.status === "ok") {
                    this.successMsg = "You found a flag!";
                    this.flag = '';
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
