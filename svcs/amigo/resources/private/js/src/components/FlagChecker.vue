<template>
    <form @submit.prevent="submit">
        <div class="field has-addons">
            <div class="control has-icons-left is-expanded">
                <input class="input" v-bind:class="{ 'is-success': successMsg, 'is-danger': errorMsg }" type="text" @keydown="clearMessages" @click="clearMessages" v-model="flag" :placeholder="description">
                <span class="icon is-small is-left" v-bind:class="{ 'has-text-success': successMsg, 'has-text-danger': errorMsg }">
	<i class="fas fa-flag"></i>
      </span>
            </div>
            <div class="control">
                <button type="submit" class="button is-info">
                    {{ action }}
                </button>
            </div>
        </div>
        <div class="field">
            <p v-if="errorMsg" class="help is-danger">{{ errorMsg }}</p>
            <p v-if="successMsg" class="help is-success">{{ successMsg }}</p>
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
                    this.successMsg = "You found a flag!"
                    this.flag = '';
                }
            }
        }
    }
</script>
